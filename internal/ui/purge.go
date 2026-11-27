package ui

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	. "github.com/egoist/mygo/ui"

	"elom/internal/core/purge"
	"elom/locales"
)

type purgeState struct {
	paths      []string
	newPath    string
	candidates []purge.Candidate
	checked    map[string]bool
	scanning   bool
	busy       bool
	scanned    bool
	progress   purge.Progress
	err        string
	finished   string
	cancel     context.CancelFunc
}

func newPurgeState() purgeState {
	return purgeState{checked: map[string]bool{}}
}

func (a *App) purgeTab(c *Context, pal palette) {
	st := &a.purge

	if st.err != "" {
		cardWith(c, 4, func() {
			Text(c, locales.CleanError(st.err)).FontSize(12.5).TextColor(appTheme.Danger)
		})
	}

	// Project roots configuration card.
	cardWith(c, 10, func() {
		Row(c).FillWidth().Gap(8).AlignItems(Center).Children(func() {
			Box(c).Size(6, 6).Radius(3).Background(pal.glow)
			capsLabel(c, locales.PurgeFoldersTitle(), quietText())
			Spacer(c)
			if ghostPill(c, locales.PurgeAddFolder()).Clicked() {
				a.choosePurgeFolder()
			}
			if !st.scanning && !st.busy && len(st.paths) > 0 {
				label := locales.PurgeScan()
				if st.scanned {
					label = locales.CleanRescan()
				}
				if primaryPill(c, label).Clicked() {
					a.startPurgeScan()
				}
			}
		})
		Text(c, locales.PurgeFoldersHint()).FontSize(sizeCaption).TextColor(fgMuted())
		if len(st.paths) == 0 {
			Text(c, locales.PurgeNoFolders()).FontSize(sizeMeta).TextColor(fgMuted())
		} else {
			rule(c)
			for i, p := range st.paths {
				i, p := i, p
				row := Row(c).FillWidth().Gap(8).AlignItems(Center).Padding(4, 2).Height(34).Radius(radiusRow)
				row.Children(func() {
					Icon(c, iconFolder).FontSize(13).TextColor(fgMuted())
					Text(c, p).Font(fontMono).FontSize(11.5).TextColor(fgPrimary()).
						Grow(1).Shrink(1).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(p)
					if iconButton(c, iconClose, locales.PurgeRemoveFolder()).Clicked() {
						a.removePurgePath(i)
					}
				})
				if row.Hovered() {
					row.Background(faintFill())
				}
			}
		}
	})

	switch {
	case st.scanning:
		cardWith(c, 10, func() {
			Row(c).FillWidth().Gap(10).AlignItems(Center).Children(func() {
				Spinner(c)
				Text(c, locales.PurgeScanning(st.progress.ScannedDirs, st.progress.Found)).
					FontSize(12.5).TextColor(appTheme.TextMuted)
				Spacer(c)
				if cancel := st.cancel; cancel != nil {
					if ghostPill(c, locales.CleanCancel()).Clicked() {
						cancel()
					}
				}
			})
			if st.progress.Current != "" {
				Text(c, st.progress.Current).Font(fontMono).FontSize(sizeCaption).TextColor(fgMuted()).
					SingleLine().Ellipsis("…")
			}
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
				a.deletePurgeSelected()
			}
		}
	})

	if len(st.candidates) == 0 {
		cardWith(c, 4, func() {
			Text(c, locales.PurgeNothing()).FontSize(13).TextColor(appTheme.TextMuted)
		})
		return
	}

	cardWith(c, 4, func() {
		for i, cand := range st.candidates {
			a.purgeCandidateRow(c, cand)
			if i < len(st.candidates)-1 {
				rowDivider(c)
			}
		}
	})
}

func (a *App) purgeCandidateRow(c *Context, cand purge.Candidate) {
	row := listRow(c)
	row.Children(func() {
		if cand.Blocked != nil {
			Text(c, "—").Width(18).TextColor(quietText())
		} else {
			checked := a.purge.checked[cand.Path]
			box := Checkbox(c, &checked, "").Label(cand.Path)
			if box.Changed() {
				a.purge.checked[cand.Path] = checked
			}
		}
		Column(c).Grow(1).Gap(2).Children(func() {
			Row(c).Gap(8).AlignItems(Center).Children(func() {
				Text(c, cand.ProjectName).FontSize(12.5).FontWeight(600).TextColor(fgPrimary()).NoWrap()
				pillTag(c, cand.Artifact, fgSecondary())
				if cand.Recent {
					pillTag(c, locales.PurgeRecentTag(), appTheme.Warning)
				}
			})
			switch {
			case cand.Blocked != nil:
				Text(c, locales.CleanBlocked(cand.Blocked.Error())).FontSize(sizeCaption).TextColor(appTheme.Warning)
			case cand.Recent:
				Text(c, locales.PurgeRecentHint()).FontSize(sizeCaption).TextColor(fgMuted())
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

func (a *App) choosePurgeFolder() {
	if a.Win == nil {
		return
	}
	go func() {
		dirs, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.Win,
			Title:     locales.PurgeAddFolder(),
			Directory: true,
		})
		if err != nil || len(dirs) == 0 {
			return
		}
		a.Win.Update(func() { a.addPurgePath(dirs[0]) })
	}()
}

func (a *App) addPurgePath(path string) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || clean == "." {
		return
	}
	for _, existing := range a.purge.paths {
		if strings.EqualFold(filepath.Clean(existing), clean) {
			return
		}
	}
	a.purge.paths = append(a.purge.paths, clean)
	if a.Purge != nil {
		if err := a.Purge.SavePaths(a.purge.paths); err != nil {
			a.purge.err = err.Error()
		}
	}
}

func (a *App) removePurgePath(index int) {
	if index < 0 || index >= len(a.purge.paths) {
		return
	}
	a.purge.paths = append(a.purge.paths[:index], a.purge.paths[index+1:]...)
	if a.Purge != nil {
		if err := a.Purge.SavePaths(a.purge.paths); err != nil {
			a.purge.err = err.Error()
		}
	}
}

func (a *App) startPurgeScan() {
	if a.purge.scanning || a.Win == nil || a.Purge == nil {
		return
	}
	roots := append([]string(nil), a.purge.paths...)
	ctx, cancel := context.WithCancel(context.Background())
	a.purge.scanning = true
	a.purge.err = ""
	a.purge.finished = ""
	a.purge.cancel = cancel
	a.purge.progress = purge.Progress{}
	go func() {
		progress := func(p purge.Progress) {
			a.Win.Update(func() { a.purge.progress = p })
		}
		found, err := a.Purge.Scan(ctx, roots, progress)
		a.Win.Update(func() {
			a.purge.scanning = false
			a.purge.scanned = true
			a.purge.cancel = nil
			a.purge.progress = purge.Progress{}
			if err != nil {
				a.purge.err = err.Error()
				return
			}
			a.purge.candidates = found
			a.purge.checked = map[string]bool{}
			for _, cand := range found {
				if cand.Blocked == nil && cand.Checked {
					a.purge.checked[cand.Path] = true
				}
			}
		})
	}()
}

func (a *App) deletePurgeSelected() {
	chosen, bytes := a.purge.selectionCandidates()
	if len(chosen) == 0 || a.purge.busy || a.Win == nil || a.Purge == nil {
		return
	}
	roots := append([]string(nil), a.purge.paths...)
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
		a.Win.Update(func() { a.purge.busy = true })
		results := a.Purge.Delete(context.Background(), roots, chosen)
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
				delete(a.purge.checked, r.Path)
			}
			kept := a.purge.candidates[:0]
			for _, cand := range a.purge.candidates {
				if !removed[cand.Path] {
					kept = append(kept, cand)
				}
			}
			a.purge.candidates = kept
			a.purge.busy = false
			a.purge.finished = locales.CleanResult(ok, failed)
			a.reload()
		})
	}()
}

func (s *purgeState) selection() (count int, bytes int64) {
	for _, cand := range s.candidates {
		if s.checked[cand.Path] && cand.Blocked == nil {
			count++
			bytes += cand.Bytes
		}
	}
	return count, bytes
}

func (s *purgeState) selectionCandidates() ([]purge.Candidate, int64) {
	var chosen []purge.Candidate
	var bytes int64
	for _, cand := range s.candidates {
		if s.checked[cand.Path] && cand.Blocked == nil {
			chosen = append(chosen, cand)
			bytes += cand.Bytes
		}
	}
	return chosen, bytes
}
