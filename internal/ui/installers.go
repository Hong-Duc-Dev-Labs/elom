package ui

import (
	"context"

	"github.com/egoist/mygo"
	. "github.com/egoist/mygo/ui"

	"elom/internal/core/installer"
	"elom/locales"
)

type installerState struct {
	candidates []installer.Candidate
	checked    map[string]bool
	scanning   bool
	busy       bool
	scanned    bool
	err        string
	finished   string
	cancel     context.CancelFunc
}

func newInstallerState() installerState {
	return installerState{checked: map[string]bool{}}
}

func (a *App) installersTab(c *Context, pal palette) {
	st := &a.installers

	if st.err != "" {
		cardWith(c, 4, func() {
			Text(c, locales.CleanError(st.err)).FontSize(12.5).TextColor(appTheme.Danger)
		})
	}

	cardWith(c, 8, func() {
		Row(c).FillWidth().Gap(8).AlignItems(Center).Children(func() {
			Box(c).Size(6, 6).Radius(3).Background(pal.glow)
			capsLabel(c, locales.InstallersTitle(), quietText())
			Spacer(c)
			if !st.scanning && !st.busy {
				label := locales.InstallersScan()
				if st.scanned {
					label = locales.CleanRescan()
				}
				if primaryPill(c, label).Clicked() {
					a.startInstallerScan()
				}
			}
		})
		Text(c, locales.InstallersHint()).FontSize(sizeCaption).TextColor(fgMuted())
	})

	switch {
	case st.scanning:
		cardWith(c, 8, func() {
			Row(c).FillWidth().Gap(10).AlignItems(Center).Children(func() {
				Spinner(c)
				Text(c, locales.InstallersScanning()).FontSize(12.5).TextColor(appTheme.TextMuted)
				Spacer(c)
				if cancel := st.cancel; cancel != nil {
					if ghostPill(c, locales.CleanCancel()).Clicked() {
						cancel()
					}
				}
			})
		})
		return
	case st.busy:
		cardWith(c, 8, func() {
			Row(c).Gap(10).AlignItems(Center).Children(func() {
				Spinner(c)
				Text(c, locales.CleanDelete()).FontSize(12.5).TextColor(appTheme.TextMuted)
			})
		})
		return
	case !st.scanned:
		return
	}

	selected, bytes := st.selection()
	var total int64
	for _, cand := range st.candidates {
		total += cand.Bytes
	}

	Row(c).FillWidth().Gap(10).AlignItems(Center).Padding(4, 0, 0, 0).Children(func() {
		Column(c).Gap(2).Children(func() {
			Text(c, locales.CleanFound(humanBytes(total))).Font(fontMono).FontSize(20).FontWeight(500).
				TextColor(appTheme.Text).NoWrap()
			Text(c, locales.CleanSelected(selected, humanBytes(bytes))).FontSize(12.5).TextColor(fgSecondary())
		})
		Spacer(c)
		if st.finished != "" {
			Text(c, st.finished).FontSize(12).TextColor(appTheme.Success)
		}
		if selected > 0 {
			if primaryPill(c, locales.CleanDelete()).MinWidth(190).Height(42).Clicked() {
				a.deleteInstallersSelected()
			}
		}
	})

	if len(st.candidates) == 0 {
		cardWith(c, 4, func() {
			Text(c, locales.InstallersNothing()).FontSize(13).TextColor(appTheme.TextMuted)
		})
		return
	}

	cardWith(c, 4, func() {
		for i, cand := range st.candidates {
			a.installerCandidateRow(c, cand)
			if i < len(st.candidates)-1 {
				rowDivider(c)
			}
		}
	})
}

func (a *App) installerCandidateRow(c *Context, cand installer.Candidate) {
	row := listRow(c)
	row.Children(func() {
		if cand.Blocked != nil {
			Text(c, "—").Width(18).TextColor(quietText())
		} else {
			checked := a.installers.checked[cand.Path]
			box := Checkbox(c, &checked, "").Label(cand.Path)
			if box.Changed() {
				a.installers.checked[cand.Path] = checked
			}
		}
		Column(c).Grow(1).Gap(2).Children(func() {
			Row(c).Gap(8).AlignItems(Center).Children(func() {
				Text(c, cand.Name).FontSize(12.5).FontWeight(600).TextColor(fgPrimary()).NoWrap()
				pillTag(c, cand.Ext, fgSecondary())
				pillTag(c, locales.InstallersAge(cand.AgeDays), fgMuted())
			})
			switch {
			case cand.Blocked != nil:
				Text(c, locales.CleanBlocked(cand.Blocked.Error())).FontSize(sizeCaption).TextColor(appTheme.Warning)
			case cand.Ext == ".exe":
				Text(c, locales.InstallersExeHint()).FontSize(sizeCaption).TextColor(appTheme.Warning)
			default:
				Text(c, cand.Path).Font(fontMono).FontSize(sizeCaption).TextColor(fgMuted()).
					SingleLine().Ellipsis("…").Tooltip(cand.Path)
			}
		})
		Text(c, humanBytes(cand.Bytes)).Font(fontMono).FontSize(sizeMeta).
			TextColor(fgSecondary()).Width(84).TextAlign(End).NoWrap()
	})
	if row.Hovered() {
		row.Background(faintFill())
	}
}

func (a *App) startInstallerScan() {
	if a.installers.scanning || a.Win == nil || a.Clean == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.installers.scanning = true
	a.installers.err = ""
	a.installers.finished = ""
	a.installers.cancel = cancel
	go func() {
		found, err := a.Clean.ScanInstallers(ctx)
		a.Win.Update(func() {
			a.installers.scanning = false
			a.installers.scanned = true
			a.installers.cancel = nil
			if err != nil {
				a.installers.err = err.Error()
				return
			}
			a.installers.candidates = found
			a.installers.checked = map[string]bool{}
			for _, cand := range found {
				if cand.Blocked == nil && cand.Checked {
					a.installers.checked[cand.Path] = true
				}
			}
		})
	}()
}

func (a *App) deleteInstallersSelected() {
	chosen, bytes := a.installers.selectionCandidates()
	if len(chosen) == 0 || a.installers.busy || a.Win == nil || a.Clean == nil {
		return
	}
	// Always Recycle Bin, never permanent delete.
	detail, confirm := cleanConfirmFor(false, humanBytes(bytes))
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             a.Win,
			Type:               mygo.MessageWarning,
			Message:            locales.CleanConfirmTitle(len(chosen)),
			Detail:             detail,
			Buttons:            []string{locales.CleanCancel(), confirm},
			DefaultButton:      1,
			CancelButton:       0,
			DestructiveButtons: []int{1},
		})
		if err != nil || res.Button != 1 {
			return
		}
		a.Win.Update(func() { a.installers.busy = true })
		results := a.Clean.DeleteInstallers(context.Background(), chosen)
		a.Win.Update(func() {
			ok, failed := 0, 0
			removed := map[string]bool{}
			for _, r := range results {
				if r.Err != "" {
					failed++
					continue
				}
				ok++
				removed[r.Path] = true
				delete(a.installers.checked, r.Path)
			}
			kept := a.installers.candidates[:0]
			for _, cand := range a.installers.candidates {
				if !removed[cand.Path] {
					kept = append(kept, cand)
				}
			}
			a.installers.candidates = kept
			a.installers.busy = false
			a.installers.finished = locales.CleanResult(ok, failed)
			a.reload()
		})
	}()
}

func (s *installerState) selection() (count int, bytes int64) {
	for _, cand := range s.candidates {
		if s.checked[cand.Path] && cand.Blocked == nil {
			count++
			bytes += cand.Bytes
		}
	}
	return count, bytes
}

func (s *installerState) selectionCandidates() ([]installer.Candidate, int64) {
	var chosen []installer.Candidate
	var bytes int64
	for _, cand := range s.candidates {
		if s.checked[cand.Path] && cand.Blocked == nil {
			chosen = append(chosen, cand)
			bytes += cand.Bytes
		}
	}
	return chosen, bytes
}
