package ui

import (
	"sort"
	"strings"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/apps"
	"elom/locales"
)

// appsPage is Mole's Apps screen: a pill of its own tabs (Uninstall, Updates,
// Startup) with a refresh button, the sort and the search on the right, and
// the tab's list filling the window below.
func (a *App) appsPage(c *Context, pal palette) {
	st := &a.software
	Row(c).FillWidth().Gap(8).AlignItems(Center).Children(func() {
		labels := []string{locales.SoftwareUninstall(), locales.SoftwareUpdates(), locales.NavStartup()}
		seg := SegmentedBase(c, &a.appsTab, len(labels))
		seg.Track.Padding(3).Radius(radiusRow).Background(RGBA(0, 0, 0, 0.12)).
			Border(1, hairline()).Label(locales.NavApps()).Children(func() {
			for i, label := range labels {
				s := seg.Segment(i).Padding(6, 12).Radius(6).Label(label)
				color := fgMuted()
				switch {
				case i == a.appsTab:
					s.Background(pal.ink[1].Alpha(0.22))
					s.Border(1, pal.ink[1].Alpha(0.40))
					color = fgPrimary()
				case s.Hovered():
					s.Background(faintFill())
				}
				s.Children(func() {
					t := Text(c, label).Font(fontMono).FontSize(11).NoWrap().TextColor(color)
					if i == a.appsTab {
						t.FontWeight(500)
					}
				})
			}
		})
		if iconButton(c, iconRefresh, locales.AppsRefresh()).Clicked() {
			a.refreshApps()
		}
		Spacer(c)
		if a.appsTab == 0 && st.loaded {
			sortLink(c, locales.AppsSortName(), st.byName, func() { st.byName = true })
			sortLink(c, locales.AppsSortSize(), !st.byName, func() { st.byName = false })
			Box(c).Width(6)
			SearchField(c, &st.query).Placeholder(locales.SoftwareSearch()).Width(210)
		}
	})

	switch a.appsTab {
	case 1:
		if !st.updatesTried && !st.checking && a.Win != nil {
			a.checkUpdates()
		}
		Scroll(c).FillWidth().Grow(1).Children(func() {
			Column(c).FillWidth().Gap(12).Children(func() { a.updatesSection(c, pal) })
		})
	case 2:
		Scroll(c).FillWidth().Grow(1).Children(func() {
			Column(c).FillWidth().Gap(12).Children(func() { a.startupPage(c, pal) })
		})
	default:
		a.uninstallTab(c, pal)
	}
}

// sortLink is one of the quiet sort choices above the app list; the chosen one
// is bright and carries the arrow.
func sortLink(c *Context, label string, on bool, choose func()) {
	b := ButtonBase(c).Padding(4, 6).Radius(6).Label(label)
	b.Children(func() {
		text := label
		color := quietText()
		if on {
			color = appTheme.Text
			if label == locales.AppsSortSize() {
				text += " ↓"
			}
		}
		t := Text(c, text).FontSize(12).NoWrap().TextColor(color)
		if on {
			t.FontWeight(600)
		}
	})
	if b.Clicked() {
		choose()
	}
}

// refreshApps reads the chosen tab's list again.
func (a *App) refreshApps() {
	switch a.appsTab {
	case 1:
		a.checkUpdates()
	case 2:
		a.startup.loaded = false
		a.loadStartup()
	default:
		a.software.loaded = false
		a.loadApps()
	}
}

// shown is the app list as the screen draws it: filtered by the search, then
// sorted by size, largest first, or by name.
func (st *softwareState) shown() []apps.App {
	list := append([]apps.App(nil), st.visible()...)
	if st.byName {
		sort.SliceStable(list, func(i, j int) bool {
			return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
		})
	} else {
		sort.SliceStable(list, func(i, j int) bool { return list[i].SizeBytes > list[j].SizeBytes })
	}
	return list
}

// uninstallTab lists the installed apps as Mole does: the app's icon, its name,
// its version and size in a monospaced line, and a box to choose it. The
// chosen app's facts and the uninstall action show below the list.
func (a *App) uninstallTab(c *Context, pal palette) {
	st := &a.software

	switch {
	case st.err != "":
		cardWith(c, 4, func() {
			Text(c, locales.SoftwareError(st.err)).FontSize(12.5).TextColor(appTheme.Danger)
		})
	case st.loading:
		Row(c).Gap(10).AlignItems(Center).Padding(10, 4).Children(func() {
			Spinner(c)
			Text(c, locales.SoftwareLoading()).FontSize(12.5).TextColor(appTheme.TextMuted)
		})
		return
	case !st.loaded:
		a.loadApps()
		Column(c).FillWidth().AlignItems(Center).Padding(28, 0, 20, 0).Gap(16).Children(func() {
			Column(c).AlignItems(Center).Gap(10).Children(func() {
				planet(c, 120, pal)
				Box(c).Width(200).Height(1).Background(hairline())
			})
			if a.Win == nil && primaryPill(c, locales.SoftwareLoad()).Clicked() {
				a.loadApps()
			}
		})
		return
	}

	for _, warning := range st.warnings {
		Text(c, warning).FontSize(11).TextColor(appTheme.Warning).NoWrap()
	}

	if st.running {
		cardWith(c, 6, func() {
			Row(c).Gap(10).AlignItems(Center).Children(func() {
				Spinner(c)
				label := locales.SoftwareUninstalling()
				if st.waiting {
					label = locales.SoftwareWaitingRemoval()
				}
				Text(c, label).FontSize(12.5).TextColor(appTheme.TextMuted)
			})
			for _, line := range tail(st.log, 4) {
				Text(c, trim(line, 110)).FontSize(11).TextColor(quietText()).NoWrap()
			}
		})
	}
	if st.uninstallErr != "" {
		cardWith(c, 4, func() {
			Text(c, locales.SoftwareUninstallFailed(st.uninstallErr)).
				FontSize(sizeMeta).TextColor(appTheme.Danger)
		})
	}
	if st.stillInstalled && !st.running {
		active := st.active
		cardWith(c, 6, func() {
			Row(c).FillWidth().Gap(10).AlignItems(Center).Children(func() {
				Text(c, locales.SoftwareStillInstalled(active.Name)).
					FontSize(sizeMeta).TextColor(appTheme.Warning).Grow(1)
				if ghostPill(c, locales.SoftwareCheckAgain()).Clicked() {
					a.recheckRemoval(active)
				}
			})
		})
	}
	if len(st.leftovers) > 0 {
		a.leftoversSection(c, st.leftovers, pal)
	}

	list := st.shown()
	var total int64
	for _, app := range list {
		total += app.SizeBytes
	}
	card(c).Grow(1).Children(func() {
		Column(c).FillWidth().Grow(1).Gap(6).Children(func() {
			Row(c).FillWidth().Gap(8).AlignItems(Center).Padding(2, 8, 0, 8).Children(func() {
				capsLabel(c, locales.AppsInstalled(), softText())
				Text(c, locales.SoftwareCount(len(list))+" · "+humanBytes(total)).
					Font(fontMono).FontSize(sizeMicro).TextColor(quietText())
				Spacer(c)
			})
			rule(c)

	if st.picked >= len(list) {
		st.picked = -1
	}
	// Selected gives the rows the keyboard: Up/Down/Home/End move the choice
	// and Enter reports it. Without it a List is not focusable and the one
	// screen that uninstalls software was mouse-only.
	st.rows.Selected = &st.picked
	st.rows.Label = func(i int) string { return list[i].Name }
	List(c, &st.rows, len(list), func(i int) {
		app := list[i]
		chosen := i == st.picked
		// The card ground is painted by the row itself: a List paints its
		// chosen row in the theme accent (the cream primary fill) behind
		// this column, and the row plus its detail must cover it whole.
		Column(c).FillWidth().Radius(radiusCard).Background(frameCard()).Children(func() {
			row := listRow(c)
			row.Children(func() {
				a.appIcon(c, app, 28, pal)
				Column(c).Grow(1).Gap(3).Children(func() {
					Text(c, trim(app.Name, 70)).FontSize(13).FontWeight(600).TextColor(fgPrimary()).NoWrap()
					facts := []string{}
					if app.Publisher != "" {
						facts = append(facts, trim(app.Publisher, 36))
					}
					if app.Version != "" {
						facts = append(facts, trim(app.Version, 24))
					}
					if len(facts) > 0 {
						Text(c, strings.Join(facts, " · ")).FontSize(sizeCaption).
							TextColor(fgMuted()).NoWrap()
					}
				})
				Text(c, humanBytes(app.SizeBytes)).Font(fontMono).FontSize(11.5).
					TextColor(fgSecondary()).NoWrap()
				checkMark(c, chosen)
				Icon(c, iconChevron).FontSize(12).TextColor(fgFaint())
			})
			switch {
			case chosen:
				// The List paints its chosen row in the theme's accent, which
				// is the cream primary fill. Cover it with an opaque ink tint
				// at the same radius, so a selected app reads as the screen's
				// planet and not as a second primary button.
				row.Background(frameCard().Mix(pal.ink[1], 0.20)).Radius(radiusCard)
			case row.Hovered():
				row.Background(faintFill())
			}
			if chosen {
				a.softwareDetail(c, app, pal)
			}
			if i < len(list)-1 {
				rowDivider(c)
			}
		})
			}).FillWidth().Grow(1).Children(func() {
				if len(list) == 0 {
					Text(c, locales.SoftwareEmpty()).FontSize(12.5).TextColor(fgMuted()).Padding(12)
				}
			})
		})
	})
}

// appIcon is the app's own icon, or a tile with its initial when Windows has
// none to give.
func (a *App) appIcon(c *Context, app apps.App, size float32, pal palette) {
	if icon := a.iconFor(app); icon != nil {
		Image(c, icon).Size(size, size)
		return
	}
	initial := "?"
	if r := []rune(strings.TrimSpace(app.Name)); len(r) > 0 {
		initial = strings.ToUpper(string(r[0]))
	}
	// No foreign hues: a missing icon is a cell of the screen's own planet
	// ink, with the initial set in the lit tone.
	Box(c).Size(size, size).Radius(size*0.26).Background(pal.ink[0]).
		Border(0.6, hairline()).AlignItems(Center).Justify(Center).
		Children(func() {
			Text(c, initial).Font(fontMono).FontSize(size * 0.42).FontWeight(600).TextColor(pal.ink[2])
		})
}

// checkMark is the square on the right of a row, as Mole's: empty, or filled
// with a check when the row is chosen.
func checkMark(c *Context, on bool) {
	box := Box(c).Size(16, 16).Radius(4).AlignItems(Center).Justify(Center)
	if on {
		box.Background(Hex("#f7f5f0")).Children(func() {
			Icon(c, iconCheck).FontSize(11).TextColor(Hex("#12161f"))
		})
		return
	}
	box.Border(1.5, RGBA(255, 255, 255, 0.35))
}
