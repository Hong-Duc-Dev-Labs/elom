package ui

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	. "github.com/egoist/mygo/ui"

	"elom/internal/core/apps"
	"elom/internal/core/winget"
	"elom/locales"
)

type softwareState struct {
	list    []apps.App
	loading bool
	loaded  bool
	err     string

	table  ListState
	picked int
	query  string

	running bool
	// waiting is true once the uninstaller exited, while Elom waits for the
	// app's registry entry to go; stillInstalled is true when it did not.
	waiting        bool
	stillInstalled bool
	uninstallErr   string
	log            []string
	leftovers      []apps.Leftover
	chosen         map[string]bool
	active         apps.App

	updates      []winget.Upgrade
	updatesTried bool
	checking     bool
	applying     string
	updatesErr   string
	updateLog    []string
	warnings     []string
	icons        map[string]*Bitmap

	// rows is the app list; byName sorts it by name instead of by size.
	rows   ListState
	byName bool
}

func newSoftwareState() softwareState {
	return softwareState{picked: -1, chosen: map[string]bool{}, icons: map[string]*Bitmap{}}
}

// iconFor returns the app's icon, reading it once per path. MyGo draws
// Explorer's icon for a file or a shell item; a failure is cached as nil so a
// missing icon is not retried every frame.
func (a *App) iconFor(app apps.App) *Bitmap {
	path := iconPath(app)
	if path == "" {
		return nil
	}
	if icon, ok := a.software.icons[path]; ok {
		return icon
	}
	png, err := mygo.Shell.FileIcon(path, 32)
	if err != nil {
		a.software.icons[path] = nil
		return nil
	}
	bitmap, err := DecodeBitmap(png)
	if err != nil {
		a.software.icons[path] = nil
		return nil
	}
	a.software.icons[path] = bitmap
	return bitmap
}

// iconPath strips the ",0" index some uninstallers append to DisplayIcon.
func iconPath(app apps.App) string {
	path := app.Icon
	if at := strings.LastIndex(path, ","); at > 0 {
		path = path[:at]
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// visibleApps filters by the search field.
func (st *softwareState) visible() []apps.App {
	query := strings.ToLower(strings.TrimSpace(st.query))
	if query == "" {
		return st.list
	}
	var out []apps.App
	for _, app := range st.list {
		if strings.Contains(strings.ToLower(app.Name), query) ||
			strings.Contains(strings.ToLower(app.Publisher), query) {
			out = append(out, app)
		}
	}
	return out
}

// updatesSection lists what winget can update, one row each, and applies one
// update at a time.
func (a *App) updatesSection(c *Context, pal palette) {
	st := &a.software
	cardWith(c, 10, func() {
		Row(c).FillWidth().Gap(8).AlignItems(Center).Children(func() {
			Box(c).Size(6, 6).Radius(3).Background(pal.glow)
			capsLabel(c, locales.SoftwareUpdates(), quietText())
			Spacer(c)
			if st.checking {
				Spinner(c)
				Text(c, locales.SoftwareChecking()).FontSize(11).TextColor(quietText())
				return
			}
			if ghostPill(c, locales.SoftwareCheckUpdates()).Clicked() {
				a.checkUpdates()
			}
		})
		if st.applying != "" {
			Text(c, locales.SoftwareApplying(st.applying)).FontSize(11.5).TextColor(softText())
			for _, line := range tail(st.updateLog, 3) {
				Text(c, trim(line, 100)).FontSize(11).TextColor(quietText()).NoWrap()
			}
		}
		if st.updatesErr != "" {
			Text(c, locales.SoftwareUpdatesError(st.updatesErr)).FontSize(11.5).TextColor(appTheme.Danger)
			return
		}
		if st.updatesTried && len(st.updates) == 0 {
			Text(c, locales.SoftwareNoUpdates()).FontSize(12).TextColor(appTheme.TextMuted)
			return
		}
		if len(st.updates) > 0 {
			rule(c)
		}
		for i := range st.updates {
			update := st.updates[i]
			row := Row(c).FillWidth().Gap(10).AlignItems(Center).Padding(6, 2).Radius(8)
			row.Children(func() {
				Column(c).Grow(1).Gap(1).Children(func() {
					Text(c, trim(update.Name, 60)).FontSize(12.5).TextColor(appTheme.Text).NoWrap()
					Text(c, update.ID).FontSize(11).TextColor(quietText()).NoWrap()
				})
				Text(c, update.Current+" → "+update.Available).FontSize(11).TextColor(softText()).NoWrap()
				updateButton := ghostPill(c, locales.SoftwareUpdate())
				if st.applying != "" {
					updateButton.Disabled(true)
				} else if updateButton.Clicked() {
					a.applyUpdate(update)
				}
			})
			if row.Hovered() {
				row.Background(faintFill())
			}
		}
	})
}

func (a *App) checkUpdates() {
	if a.software.checking || a.Win == nil {
		return
	}
	a.software.checking = true
	a.software.updatesErr = ""
	go func() {
		updates, err := a.Software.Updates(context.Background())
		a.Win.Update(func() {
			a.software.checking = false
			a.software.updatesTried = true
			if err != nil {
				a.software.updatesErr = err.Error()
				return
			}
			a.software.updates = updates
		})
	}()
}

func (a *App) applyUpdate(update winget.Upgrade) {
	if a.software.applying != "" {
		return
	}
	a.software.applying = update.Name
	a.software.updateLog = nil
	a.software.updatesErr = ""
	go func() {
		err := a.Software.ApplyUpdate(context.Background(), update.ID, func(line string) {
			a.Win.Update(func() {
				a.software.updateLog = append(a.software.updateLog, line)
				if len(a.software.updateLog) > 40 {
					a.software.updateLog = a.software.updateLog[len(a.software.updateLog)-40:]
				}
			})
		})
		a.Win.Update(func() {
			a.software.applying = ""
			if err != nil {
				a.software.updatesErr = err.Error()
				return
			}
			a.checkUpdates()
		})
	}()
}

// softwareDetail is the panel under a chosen app row: the facts the list
// does not carry, and the two actions that use them.
func (a *App) softwareDetail(c *Context, app apps.App, pal palette) {
	Box(c).FillWidth().Padding(8, 12, 12, 52).Radius(radiusRow).
		Background(RGBA(255, 255, 255, 0.03)).Children(func() {
		Column(c).FillWidth().Gap(6).Children(func() {
			field := func(label, value string) {
				Row(c).FillWidth().Gap(12).AlignItems(Center).Children(func() {
					Text(c, label).FontSize(sizeCaption).TextColor(fgMuted()).Width(110).NoWrap()
					Text(c, value).Font(fontMono).FontSize(sizeCaption).
						TextColor(fgSecondary()).NoWrap()
				})
			}
			if app.Publisher != "" {
				field(locales.AppsPublisher(), app.Publisher)
			}
			if app.Version != "" {
				field(locales.AppsVersion(), app.Version)
			}
			if app.InstallLocation != "" {
				field(locales.AppsLocation(), app.InstallLocation)
			}
			if app.Kind != "" {
				field(locales.AppsKind(), string(app.Kind))
			}
			Row(c).FillWidth().Gap(8).AlignItems(Center).Padding(4, 0, 0, 0).Children(func() {
				if app.InstallLocation != "" {
					location := app.InstallLocation
					if ghostPill(c, locales.AnalyzeReveal()).Clicked() {
						mygo.Shell.ShowItemInFolder(location)
					}
				}
				Spacer(c)
				if app.UninstallCommand() != "" {
					if a.software.running {
						primaryPill(c, locales.SoftwareUninstall()).Disabled(true)
					} else if primaryPill(c, locales.SoftwareUninstall()).Clicked() {
						a.uninstallApp(app)
					}
				}
			})
			if app.UninstallCommand() == "" {
				hintText(c, locales.SoftwareNoUninstaller())
			}
		})
	})
}

func (a *App) leftoversSection(c *Context, leftovers []apps.Leftover, pal palette) {
	cardWith(c, 10, func() {
		cardHead(c, pal, locales.SoftwareLeftovers(), "")
		rule(c)
		for _, leftover := range leftovers {
			row := Row(c).FillWidth().Gap(10).AlignItems(Center).Padding(7, 8).Radius(8)
			row.Children(func() {
				checked := a.software.chosen[leftover.Path]
				box := Checkbox(c, &checked, "")
				if box.Changed() {
					a.software.chosen[leftover.Path] = checked
				}
				Column(c).Grow(1).Gap(1).Children(func() {
					Text(c, leftover.Path).FontSize(12.5).TextColor(appTheme.Text).NoWrap()
					Text(c, leftover.Confidence+" — "+leftover.Reason).FontSize(11).TextColor(quietText())
				})
				Text(c, humanBytes(leftover.Bytes)).FontSize(12).TextColor(softText()).NoWrap()
			})
			if row.Hovered() {
				row.Background(faintFill())
			}
		}
		if primaryPill(c, locales.SoftwareTrashLeftovers()).Clicked() {
			a.trashLeftovers()
		}
	})
}

func (a *App) loadApps() {
	if a.software.loading || a.Win == nil {
		return
	}
	a.software.loading = true
	a.software.err = ""
	go func() {
		list, warnings, err := a.Software.List(context.Background())
		a.Win.Update(func() {
			a.software.loading = false
			a.software.loaded = true
			a.software.warnings = warnings
			if err != nil {
				a.software.err = err.Error()
				return
			}
			a.software.list = list
			a.software.picked = -1
		})
	}()
}

func (a *App) uninstallApp(app apps.App) {
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             a.Win,
			Type:               mygo.MessageWarning,
			Message:            locales.SoftwareConfirmTitle(app.Name),
			Detail:             locales.SoftwareConfirmDetail(),
			Buttons:            []string{locales.CleanCancel(), locales.SoftwareUninstall()},
			DefaultButton:      1,
			CancelButton:       0,
			DestructiveButtons: []int{1},
		})
		if err != nil || res.Button != 1 {
			return
		}
		a.Win.Update(func() {
			a.software.running = true
			a.software.waiting = false
			a.software.stillInstalled = false
			a.software.uninstallErr = ""
			a.software.log = nil
			a.software.leftovers = nil
			a.software.chosen = map[string]bool{}
			a.software.active = app
		})
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		runErr := a.Software.Uninstall(ctx, app, func(line string) {
			a.Win.Update(func() {
				a.software.log = append(a.software.log, line)
				if len(a.software.log) > 200 {
					a.software.log = a.software.log[len(a.software.log)-200:]
				}
			})
		})
		if runErr != nil {
			a.Win.Update(func() {
				a.software.running = false
				a.software.uninstallErr = runErr.Error()
			})
			return
		}
		a.Win.Update(func() { a.software.waiting = true })
		a.finishUninstall(ctx, app, removalGrace)
	}()
}

// removalGrace is how long Elom waits, after the uninstaller exits, for the
// app's registry entry to go. A vendor dialog left open longer than this ends
// in the "still installed" notice and its Check again button.
const removalGrace = 3 * time.Minute

// finishUninstall offers leftovers only once the app's uninstall entry is gone.
// An uninstaller that exits 0 may have relaunched itself from %TEMP% or been
// cancelled; offering the files of an app that is still installed would send a
// working program to the Recycle Bin. It runs off the UI thread.
func (a *App) finishUninstall(ctx context.Context, app apps.App, grace time.Duration) {
	removed := a.Software.AwaitRemoval(ctx, app, grace, 2*time.Second)
	var leftovers []apps.Leftover
	if removed {
		leftovers = a.Software.Leftovers(app)
	}
	a.Win.Update(func() {
		a.software.running = false
		a.software.waiting = false
		a.software.stillInstalled = !removed
		a.software.leftovers = leftovers
		a.software.chosen = map[string]bool{}
	})
}

// recheckRemoval looks once more whether the app is gone, after the user
// finished an uninstaller that outlived the grace period.
func (a *App) recheckRemoval(app apps.App) {
	if a.software.running {
		return
	}
	a.software.running = true
	a.software.waiting = true
	a.software.stillInstalled = false
	go a.finishUninstall(context.Background(), app, 0)
}

func (a *App) trashLeftovers() {
	var paths []string
	for path, on := range a.software.chosen {
		if on {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return
	}
	var total int64
	for _, leftover := range a.software.leftovers {
		if a.software.chosen[leftover.Path] {
			total += leftover.Bytes
		}
	}
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             a.Win,
			Type:               mygo.MessageWarning,
			Message:            locales.CleanConfirmTitle(len(paths)),
			Detail:             locales.CleanConfirmDetail(humanBytes(total)),
			Buttons:            []string{locales.CleanCancel(), locales.CleanDelete()},
			DefaultButton:      1,
			CancelButton:       0,
			DestructiveButtons: []int{1},
		})
		if err != nil || res.Button != 1 {
			return
		}
		results := a.Clean.DeletePaths(context.Background(), paths)
		a.Win.Update(func() {
			kept := a.software.leftovers[:0]
			for _, leftover := range a.software.leftovers {
				removed := false
				for _, r := range results {
					if r.Path == leftover.Path && r.Err == "" {
						removed = true
					}
				}
				if !removed {
					kept = append(kept, leftover)
				}
			}
			a.software.leftovers = kept
			a.software.chosen = map[string]bool{}
		})
	}()
}

func tail(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}
