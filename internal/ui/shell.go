// Package ui draws Elom's windows with MyGo's native UI: a pill of tabs and
// one screen at a time. It imports the framework's widget package with a dot
// import so call sites read as the framework's documentation writes them,
// and because the package is itself named ui.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	. "github.com/egoist/mygo/ui"

	"elom/internal/core/maint"
	"elom/internal/core/metrics"
	"elom/internal/core/oplog"
	"elom/internal/core/safety"
	"elom/internal/core/settings"
	"github.com/egoist/mygo/i18n"
	"elom/internal/svc"
	"elom/locales"
)

// App is the state the window draws and the services it talks to. The view
// is a function of it: MyGo builds frames from it on the main thread.
type App struct {
	// Screen is the chosen tab's screen.
	Screen string

	Guard   *safety.Guard
	Store   *settings.Store
	Log     *oplog.Log
	DataDir string

	// Win is the main window, once it exists.
	Win *mygo.Window

	Clean       *svc.Clean
	clean       cleanState
	analyze     analyzeState

	Software *svc.Software
	software softwareState
	optimize optimizeState
	Startup  svc.Startup
	startup  startupState
	// appsTab is the Apps screen's tab: Uninstall, Updates or Startup.
	appsTab int

	Status        *svc.Status
	status        metrics.Snapshot
	statusStarted bool
	statusCancel  context.CancelFunc
	keepOn        bool
	release       func()
	procTable     ListState
	procPicked    int

	// Elevate runs maintenance with administrator rights; tests replace it.
	Elevate func(context.Context, []maint.Op) ([]maint.Result, error)

	language string
	// washFrom is the screen the wash is fading from, and washing says the
	// 320 ms fade is running.
	washFrom string
	washing  bool
	washAt   time.Time
	cleanScreen   *mygo.Window
	cleanOpenedAt time.Time
	whitelist     []string
	newPattern    string

	Settings     settings.Settings
	SettingsErr  string
	WhitelistErr string
}

// New returns the app ready to draw.
func New(dataDir string, store *settings.Store, log *oplog.Log) *App {
	registerFonts()
	a := &App{
		Screen:      firstScreen(),
		Guard:       safety.New(),
		Store:       store,
		Log:         log,
		DataDir:     dataDir,
		Clean:       svc.NewClean(safety.New(), store, log, nil, shellTrasher{}),
		clean:       newCleanState(),
		analyze:     newAnalyzeState(),
		Elevate:     svc.RunElevated,
		Status:      svc.NewStatus(),
		procPicked:  -1,
		software:    newSoftwareState(),
		optimize:    newOptimizeState(),
		startup:     newStartupState(),
	}
	home, local, roaming, programData := svc.RootsFromUser()
	a.Software = &svc.Software{
		Home:        home,
		Local:       local,
		Roaming:     roaming,
		ProgramData: programData,
		Exclude:     func(path string) bool { return a.Guard.Check(path) != nil },
	}
	a.reload()
	return a
}

// update runs fn on the UI thread, or directly when there is no window yet
// (tests), so state changes never touch a frame from another goroutine.
func (a *App) update(fn func()) {
	if a.Win != nil {
		a.Win.Update(fn)
		return
	}
	fn()
}

// reload reads the settings and the whitelist, keeping the errors so the
// Settings screen can show them rather than silently using defaults.
func (a *App) reload() {
	if a.Store == nil {
		return
	}
	loaded, err := a.Store.Load()
	if err != nil {
		a.Settings = settings.Default()
		a.SettingsErr = err.Error()
	} else {
		a.Settings = loaded
		a.SettingsErr = ""
	}
	patterns, err := a.Store.Whitelist()
	if err != nil {
		a.WhitelistErr = err.Error()
		a.whitelist = nil
	} else {
		a.WhitelistErr = ""
		a.whitelist = patterns
	}
}

// View builds the whole window: the tinted background, the floating tabs and
// the chosen screen.
func (a *App) View(c *Context) {
	c.SetTheme(appTheme)
	pal := a.framePalette(c)

	Box(c).Fill().Background(bgBase).Font(fontBody).Draw(func(p *Painter, r Rect) {
		// Mole's wash: 217/255 at the top, 107/255 from the middle down.
		half := r.H / 2
		p.FillGradient(Rect{X: r.X, Y: r.Y, W: r.W, H: half}, LinearGradient{
			Angle: 180,
			From:  pal.wash.Alpha(217.0 / 255.0),
			To:    pal.wash.Alpha(107.0 / 255.0),
		}, 0)
		p.Fill(Rect{X: r.X, Y: r.Y + half, W: r.W, H: r.H - half}, pal.wash.Alpha(107.0/255.0), 0)
		p.FillRadialGradient(r, RadialGradient{
			Center: Point{X: 0.5, Y: 0},
			Radius: r.W * 0.62,
			Stops: []GradientStop{
				{Color: pal.glow.Alpha(0.12), At: 0},
				{Color: Transparent, At: 1},
			},
		}, 0)
	}).Children(func() {
		Column(c).Fill().Children(func() {
			a.topBar(c, pal)
			a.page(c, pal)
		})
	})
}

// framePalette is the screen's palette, blended with the one it is coming
// from while the tab wash cross-fades. The fade is skipped headless, where
// there is no frame clock.
func (a *App) framePalette(c *Context) palette {
	pal := paletteFor(a.Screen)
	if a.Win == nil || c.Preferences().ReduceMotion {
		frameWash = pal.wash
		a.washFrom = a.Screen
		a.washing = false
		return pal
	}
	if a.washFrom == "" {
		a.washFrom = a.Screen
	}
	if a.washFrom == a.Screen {
		a.washing = false
		frameWash = pal.wash
		return pal
	}
	if !a.washing {
		a.washing = true
		a.washAt = time.Now()
	}
	t := float32(time.Since(a.washAt)) / float32(320*time.Millisecond)
	if t < 1 {
		pal = mixPalette(paletteFor(a.washFrom), pal, t)
		c.AnimationFrame()
	} else {
		a.washing = false
		a.washFrom = a.Screen
	}
	frameWash = pal.wash
	return pal
}

// topBar holds the navigation pill, centered in the window's title bar as
// Mole's: the window controls sit over its right end, and the empty part of
// the bar drags the window.
func (a *App) topBar(c *Context, pal palette) {
	bar := c.TitleBar()
	side := max(bar.Right, bar.Left) + 12
	Row(c).FillWidth().Height(max(bar.Height, 32)+26).Padding(14, side, 10, side).
		AlignItems(Center).DragWindow().Children(func() {
		Spacer(c)
		a.nav(c, pal)
		Spacer(c)
	})
}

// nav is the pill of tabs: the app's planet, the five screens, and a gear for
// Settings. TabsBase brings the keyboard, the focus ring and the screen reader
// along; the look is ours.
func (a *App) nav(c *Context, pal palette) {
	screens := allScreens()
	idx := screenIndex(a.Screen)

	bar := Row(c).Padding(4).Radius(radiusCard).
		Background(RGBA(0, 0, 0, 0.18)).
		Border(1, hairline()).
		AlignItems(Center)
	bar.Children(func() {
		Box(c).Size(28, 28).Margin(0, 6, 0, 2).Children(func() {
			appMark(c, 28, pal)
		})
		tabs := TabsBase(c, &idx, len(screens))
		tabs.List.Gap(2).AlignItems(Center).Children(func() {
			for i, screen := range screens {
				s := screen
				tab := tabs.Tab(i).Radius(radiusRow).Label(s.Label())
				if s.ID == "settings" {
					tab.Size(32, 32).AlignItems(Center).Justify(Center).Margin(0, 0, 0, 4)
				} else {
					tab.Padding(6, 14)
				}
				switch {
				case i == idx:
					// The screen's own ink is the one selected surface; the
					// old cream chip made the tabs a second white app. The
					// mix toward the lit tone keeps the label above 4.5:1.
					tab.Background(pal.ink[1].Mix(pal.ink[2], 0.30))
				case tab.Hovered():
					tab.Background(RGBA(255, 255, 255, 0.08))
				}
				tab.Children(func() {
					color := RGBA(255, 255, 255, 0.72)
					if i == idx {
						color = bgBase
					}
					if s.ID == "settings" {
						Icon(c, iconSettings).FontSize(15).TextColor(color)
						return
					}
					text := Text(c, s.Label()).Font(fontMono).FontSize(11.5).LetterSpacing(0.4).
						NoWrap().TextColor(color)
					if i == idx {
						text.FontWeight(500)
					}
				})
			}
		})
		if tabs.List.Changed() && idx >= 0 && idx < len(screens) {
			a.Screen = screens[idx].ID
		}
	})
}

// fullHeight reports the screens that lay out to the window's height, with
// their own scrolling lists, rather than scrolling as a page.
func fullHeight(id string) bool {
	return id == "software" || id == "analyze"
}

func (a *App) page(c *Context, pal palette) {
	r := screenFor(a.Screen)
	body := func() {
		switch r.ID {
		case "clean":
			a.cleanPage(c, pal)
		case "analyze":
			a.analyzePage(c, pal)
		case "status":
			a.statusPage(c, pal)
		case "software":
			a.appsPage(c, pal)
		case "optimize":
			a.optimizePage(c, pal)
		case "settings":
			a.heading(c, r, pal)
			a.settingsPage(c, pal)
		}
	}
	if fullHeight(r.ID) {
		Column(c).FillWidth().Grow(1).Padding(4, 24, 20, 24).Gap(12).Children(body)
		return
	}
	// The hero screens take the window's height, so their stage centers in
	// the viewport instead of sitting at the top of a scrolling page.
	if r.ID == "clean" && a.clean.heroOnly() {
		Column(c).FillWidth().Grow(1).Padding(4, 24, 20, 24).Children(func() {
			Column(c).FillWidth().MaxWidth(1100).Grow(1).Justify(Center).Children(body)
		})
		return
	}
	Scroll(c).FillWidth().Grow(1).Children(func() {
		Column(c).FillWidth().AlignItems(Center).Padding(4, 24, 32, 24).Children(func() {
			Column(c).FillWidth().MaxWidth(1100).Gap(14).Children(body)
		})
	})
}

// heading is the screen's name and its one-line promise. Settings carries
// its moon beside them: the one screen without a hero still has a body.
func (a *App) heading(c *Context, r Screen, pal palette) {
	Row(c).FillWidth().Padding(14, 2, 2, 2).AlignItems(Center).Gap(24).Children(func() {
		Column(c).Grow(1).Gap(4).Children(func() {
			Text(c, r.Label()).FontSize(24).FontWeight(600).TextColor(appTheme.Text)
			Text(c, r.Summary()).FontSize(13).TextColor(appTheme.TextMuted).MaxWidth(640)
		})
		planet(c, 84, pal)
	})
}

// settingsPage is one card of rows, Mole's settings shape: a label and its
// hint on the left, the control on the right, a hairline between rows.
func (a *App) settingsPage(c *Context, pal palette) {
	section := func(title string) {
		Text(c, strings.ToUpper(title)).FontSize(sizeFootnote).FontWeight(700).
			LetterSpacing(0.8).TextColor(fgMuted()).NoWrap().Padding(14, 6, 4, 6)
	}
	row := func(label, desc string, control func()) {
		Row(c).FillWidth().MinHeight(44).Padding(4, 6).AlignItems(Center).Gap(12).Children(func() {
			Column(c).Grow(1).Gap(2).Children(func() {
				Text(c, label).FontSize(12.5).TextColor(fgPrimary())
				if desc != "" {
					Text(c, desc).FontSize(sizeCaption).TextColor(fgMuted())
				}
			})
			control()
		})
		rule(c)
	}
	block := func(build func()) {
		Box(c).FillWidth().Padding(4, 6).Children(build)
		rule(c)
	}

	card(c).MaxWidth(860).Children(func() {
		Column(c).FillWidth().Children(func() {
			section(locales.Preferences())

			permanent := a.Settings.PermanentDelete
			row(locales.PermanentDelete(onOff(permanent)),
				locales.ScanLimits(a.Settings.MaxScanDepth, a.Settings.MaxScanEntries),
				func() {
					if toggle(c, &permanent, locales.PermanentDelete(onOff(permanent))).Changed() {
						a.Settings.PermanentDelete = permanent
						if err := a.Store.Save(a.Settings); err != nil {
							a.SettingsErr = err.Error()
						} else {
							a.SettingsErr = ""
						}
					}
				})

			if a.SettingsErr != "" || a.WhitelistErr != "" {
				block(func() {
					if a.SettingsErr != "" {
						Text(c, locales.SettingsUnreadable(a.SettingsErr)).FontSize(sizeCaption).TextColor(appTheme.Danger)
					}
					if a.WhitelistErr != "" {
						Text(c, locales.WhitelistUnreadable(a.WhitelistErr)).FontSize(sizeCaption).TextColor(appTheme.Danger)
					}
				})
			}

			labels := []string{locales.LanguageSystem()}
			tags := []string{""}
			chosen := 0
			for _, tag := range mygo.App.Languages() {
				labels = append(labels, i18n.Autonym(tag))
				tags = append(tags, tag)
			}
			for i, tag := range tags {
				if tag == a.language {
					chosen = i
				}
			}
			prev := chosen
			row(locales.SettingsLanguage(), "", func() {
				seg := SegmentedBase(c, &chosen, len(labels))
				seg.Track.Padding(3).Radius(radiusRow).Background(RGBA(0, 0, 0, 0.12)).
					Border(1, hairline()).Label(locales.SettingsLanguage()).Children(func() {
					for i, label := range labels {
						s := seg.Segment(i).Padding(6, 12).Radius(6).Label(label)
						color := fgMuted()
						switch {
						case i == chosen:
							s.Background(pal.ink[1].Alpha(0.32))
							s.Border(1, pal.ink[1].Alpha(0.55))
							color = fgPrimary()
						case s.Hovered():
							s.Background(faintFill())
						}
						s.Children(func() {
							t := Text(c, label).Font(fontMono).FontSize(11).NoWrap().TextColor(color)
							if i == chosen {
								t.FontWeight(500)
							}
						})
					}
				})
				if chosen != prev && chosen >= 0 && chosen < len(tags) {
					a.language = tags[chosen]
					mygo.App.SetLocale(a.language)
				}
			})

			section(locales.WhitelistTitle())
			block(func() {
				Text(c, locales.WhitelistHint()).FontSize(sizeCaption).TextColor(fgMuted())
				Row(c).Gap(8).AlignItems(Center).Children(func() {
					field := TextInput(c, &a.newPattern).Placeholder(`C:\Users\me\AppData\Local\Keep`).
						Label(locales.WhitelistTitle()).Width(460)
					submitted := field.Submitted()
					if ghostPill(c, locales.WhitelistAdd()).Clicked() || submitted {
						a.addWhitelistPattern()
					}
				})
				if len(a.whitelist) == 0 {
					Text(c, locales.WhitelistEmpty()).FontSize(sizeCaption).TextColor(fgMuted())
				}
				for i, pattern := range a.whitelist {
					i, pattern := i, pattern
					r := Row(c).FillWidth().Gap(8).AlignItems(Center).Padding(4, 0).Height(36)
					r.Children(func() {
						Icon(c, iconFolder).FontSize(13).TextColor(fgMuted())
						Text(c, pattern).Font(fontMono).FontSize(11.5).TextColor(fgPrimary()).NoWrap().Grow(1)
						if iconButton(c, iconClose, locales.WhitelistRemove()).Clicked() {
							a.removeWhitelistPattern(i)
						}
					})
					if r.Hovered() {
						r.Background(faintFill())
					}
				}
				if a.WhitelistErr != "" {
					Text(c, locales.WhitelistSaveError(a.WhitelistErr)).FontSize(sizeCaption).TextColor(appTheme.Danger)
				}
			})

			section(locales.ProtectedFolders())
			block(func() {
				Text(c, locales.ProtectedHint()).FontSize(sizeCaption).TextColor(fgMuted()).Margin(0, 0, 6, 0)
				for _, root := range a.Guard.AllowRoots() {
					Text(c, root).Font(fontMono).FontSize(11).TextColor(fgSecondary()).NoWrap()
				}
			})

			section(locales.OperationLog())
			block(func() {
				if a.Log == nil {
					Text(c, locales.LogUnavailable()).FontSize(sizeCaption).TextColor(fgMuted())
					return
				}
				Text(c, a.Log.Path()).Font(fontMono).FontSize(11).TextColor(fgSecondary()).NoWrap()
				if err := a.Log.RequireWritable(); err != nil {
					Text(c, locales.LogNotWritable(err.Error())).FontSize(sizeCaption).TextColor(appTheme.Danger)
				} else {
					Text(c, locales.LogWritable()).FontSize(sizeCaption).TextColor(appTheme.Success)
				}
			})

			row(locales.CleanScreenButton(), locales.CleanScreenRow(), func() {
				if ghostPill(c, locales.CleanScreenOpen()).Clicked() {
					a.showCleanScreen()
				}
			})

			section(locales.LogRecent())
			block(func() {
				records, err := oplog.Read(a.Log.Path())
				if err != nil && len(records) == 0 {
					Text(c, err.Error()).FontSize(sizeCaption).TextColor(appTheme.Danger)
					return
				}
				if len(records) == 0 {
					Text(c, locales.LogEmpty()).FontSize(sizeCaption).TextColor(fgMuted())
					return
				}
				start := len(records) - 12
				if start < 0 {
					start = 0
				}
				for _, record := range records[start:] {
					Text(c, fmt.Sprintf("%s  %-12s %-10s %s", record.Time.Format("15:04:05"), record.Op, record.Result, trim(record.Path, 80))).
						Font(fontMono).FontSize(11).TextColor(fgMuted()).NoWrap()
				}
			})
		})
	})

	Row(c).FillWidth().AlignItems(Center).Children(func() {
		Spacer(c)
		Text(c, versionLabel()).FontSize(sizeFootnote).TextColor(fgMuted())
	})
}

// addWhitelistPattern saves the pattern in the editor, ignoring duplicates and
// blanks.
func (a *App) addWhitelistPattern() {
	pattern := strings.TrimSpace(a.newPattern)
	if pattern == "" {
		return
	}
	for _, existing := range a.whitelist {
		if strings.EqualFold(existing, pattern) {
			a.newPattern = ""
			return
		}
	}
	a.whitelist = append(a.whitelist, pattern)
	a.newPattern = ""
	a.saveWhitelist()
}

// removeWhitelistPattern drops the pattern at index and saves.
func (a *App) removeWhitelistPattern(index int) {
	if index < 0 || index >= len(a.whitelist) {
		return
	}
	a.whitelist = append(a.whitelist[:index], a.whitelist[index+1:]...)
	a.saveWhitelist()
}

func (a *App) saveWhitelist() {
	if a.Store == nil {
		return
	}
	if err := a.Store.SaveWhitelist(a.whitelist); err != nil {
		a.WhitelistErr = err.Error()
		return
	}
	a.WhitelistErr = ""
}

// versionLabel says the app's version, or "dev" when it is not running as a
// packaged app (tests, go run).
func versionLabel() string {
	if v := mygo.App.Version(); v != "" {
		return "v" + v
	}
	return "dev"
}

func onOff(v bool) string {
	if v {
		return locales.On()
	}
	return locales.Off()
}
