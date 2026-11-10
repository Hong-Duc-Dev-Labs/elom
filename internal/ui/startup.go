package ui

import (
	"context"
	"errors"
	"strconv"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/maint"
	"elom/internal/core/startup"
	"elom/internal/svc"
	"elom/locales"
)

type startupState struct {
	entries []startup.Entry
	loading bool
	loaded  bool
	err     string
	table   ListState
	picked  int
}

func newStartupState() startupState { return startupState{picked: -1} }

func (a *App) startupPage(c *Context, pal palette) {
	st := &a.startup

	switch {
	case st.err != "":
		cardWith(c, 4, func() {
			Text(c, locales.StartupError(st.err)).FontSize(12.5).TextColor(appTheme.Danger)
		})
	case st.loading:
		cardWith(c, 8, func() {
			Row(c).Gap(10).AlignItems(Center).Children(func() {
				Spinner(c)
				Text(c, locales.StartupLoading()).FontSize(12.5).TextColor(appTheme.TextMuted)
			})
		})
		return
	case !st.loaded:
		a.loadStartup()
		Column(c).FillWidth().AlignItems(Center).Padding(28, 0, 20, 0).Gap(16).Children(func() {
			planet(c, 120, pal)
			if a.Win == nil && primaryPill(c, locales.StartupLoad()).Clicked() {
				a.loadStartup()
			}
		})
		return
	}

	if len(st.entries) == 0 {
		cardWith(c, 4, func() {
			Text(c, locales.StartupEmpty()).FontSize(13).TextColor(appTheme.TextMuted)
		})
		return
	}

	st.table.Selected = &st.picked
	cardWith(c, 0, func() {
		Column(c).FillWidth().Gap(1).Children(func() {
			for i := range st.entries {
				entry := &st.entries[i]
				i := i
				row := listRow(c)
				row.Children(func() {
					Icon(c, iconStartup).FontSize(15).TextColor(fgMuted())
					Column(c).Grow(1).Gap(2).Children(func() {
						Text(c, trim(entry.Name, 60)).FontSize(12.5).TextColor(fgPrimary()).NoWrap()
						Text(c, trim(entry.Command, 100)).Font(fontMono).FontSize(10.5).
							TextColor(fgMuted()).NoWrap()
					})
					Column(c).Gap(2).AlignItems(End).Children(func() {
						Text(c, entry.Source).FontSize(sizeCaption).TextColor(fgSecondary()).NoWrap()
						state := locales.StartupEnabled()
						if !entry.Enabled {
							state = locales.StartupDisabled()
						}
						if !entry.Writable {
							state = entry.Note
						}
						Text(c, trim(state, 60)).FontSize(sizeCaption).TextColor(fgMuted()).NoWrap()
					})
					if entry.Writable {
						on := entry.Enabled
						sw := toggle(c, &on)
						if sw.Changed() {
							a.toggleStartup(i, on)
						}
					} else {
						Text(c, "—").Width(34).TextColor(fgMuted())
					}
				})
				if row.Hovered() {
					row.Background(faintFill())
				}
				if i < len(st.entries)-1 {
					rowDivider(c)
				}
			}
		})
	})
}

func (a *App) loadStartup() {
	if a.startup.loading || a.Win == nil {
		return
	}
	a.startup.loading = true
	a.startup.err = ""
	go func() {
		list, err := a.Startup.List()
		a.Win.Update(func() {
			a.startup.loading = false
			a.startup.loaded = true
			if err != nil {
				a.startup.err = err.Error()
				return
			}
			a.startup.entries = list
		})
	}()
}

// toggleStartup writes the StartupApproved flag and reflects the result. User
// entries are written directly; machine-wide ones need the elevated job,
// because HKLM is not writable by a normal user.
func (a *App) toggleStartup(index int, on bool) {
	if index < 0 || index >= len(a.startup.entries) {
		return
	}
	entry := a.startup.entries[index]
	if entry.Scope != "user" {
		a.toggleStartupElevated(entry, on)
		return
	}
	if err := a.Startup.SetEnabled(entry, on); err != nil {
		a.startup.err = err.Error()
		return
	}
	a.startup.entries[index].Enabled = on
	a.startup.err = ""
}

// startupOp is the job operation that changes a machine-wide entry.
func startupOp(entry startup.Entry, on bool) maint.Op {
	return maint.Op{
		Name: "startup.set-hklm",
		Args: map[string]string{
			"name":    entry.Name,
			"enabled": strconv.FormatBool(on),
			"scope":   "machine",
		},
	}
}

// toggleStartupElevated asks the elevated process to write a machine-wide flag.
func (a *App) toggleStartupElevated(entry startup.Entry, on bool) {
	op := startupOp(entry, on)
	go func() {
		results, err := a.Elevate(context.Background(), []maint.Op{op})
		a.update(func() {
			switch {
			case errors.Is(err, svc.ErrCancelled):
				a.startup.err = locales.OptimizeCancelled()
			case err != nil:
				a.startup.err = err.Error()
			case len(results) > 0 && !results[0].OK:
				a.startup.err = results[0].Err
			default:
				a.startup.err = ""
				if i := a.startupIndex(entry.Name); i >= 0 {
					a.startup.entries[i].Enabled = on
				}
			}
		})
	}()
}

// startupIndex finds an entry by name, since the list may have been reloaded
// while the elevated process ran.
func (a *App) startupIndex(name string) int {
	for i, entry := range a.startup.entries {
		if entry.Name == name {
			return i
		}
	}
	return -1
}
