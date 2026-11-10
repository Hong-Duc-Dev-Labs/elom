package ui

import (
	"context"
	"fmt"

	"github.com/egoist/mygo"
	. "github.com/egoist/mygo/ui"

	"elom/internal/core/scan"
	"elom/locales"
)

// shellTrasher sends a path to the Recycle Bin through MyGo, which fails
// rather than deleting for good when the bin cannot take it.
type shellTrasher struct{}

func (shellTrasher) Trash(path string) error { return mygo.Shell.TrashItem(path) }

// cleanState is the Clean screen's state. It is only touched on the main
// thread; the scan goroutine publishes through Window.Update.
type cleanState struct {
	candidates []scan.Candidate
	checked    map[string]bool
	scanning   bool
	busy       bool
	scanned    bool
	// reviewing shows the list of what was found; the hero alone shows
	// until the user asks for it, as Mole's "Review results".
	reviewing bool
	progress   scan.Progress
	err        string
	finished   string
	cancel     context.CancelFunc
}

func newCleanState() cleanState {
	return cleanState{checked: map[string]bool{}}
}

// heroOnly reports whether the screen is the stage rather than the review
// list: the page branch gives it the window's height so it can center.
func (s *cleanState) heroOnly() bool {
	return !s.reviewing || len(s.candidates) == 0
}

// cleanPage draws the review screen: what was found, what is selected, and
// the one action that removes it.
func (a *App) cleanPage(c *Context, pal palette) {
	st := &a.clean

	if st.err != "" {
		cardWith(c, 4, func() {
			Text(c, locales.CleanError(st.err)).FontSize(12.5).TextColor(appTheme.Danger)
		})
	}

	switch {
	case st.scanning:
		a.cleanHero(c, pal, 190, func() {
			Row(c).Gap(10).AlignItems(Center).Children(func() {
				Spinner(c)
				Text(c, locales.CleanScanning(st.progress.Done, st.progress.Total)).FontSize(13).TextColor(appTheme.TextMuted)
			})
			fraction := float32(0)
			if st.progress.Total > 0 {
				fraction = float32(st.progress.Done) / float32(st.progress.Total)
			}
			bar(c, fraction, pal.glow, 5).MaxWidth(360)
			if cancel := st.cancel; cancel != nil {
				if ghostPill(c, locales.CleanCancel()).Clicked() {
					cancel()
				}
			}
		})
		return
	case st.busy:
		a.cleanHero(c, pal, 190, func() {
			Row(c).Gap(10).AlignItems(Center).Children(func() {
				Spinner(c)
				Text(c, locales.CleanDelete()).FontSize(13).TextColor(appTheme.TextMuted)
			})
		})
		return
	case !st.scanned:
		a.cleanHero(c, pal, 232, func() {
			Text(c, locales.CleanNote()).FontSize(13.5).TextColor(appTheme.TextMuted).
				MaxWidth(460).TextAlign(Center).LineHeight(1.55)
			if primaryPill(c, locales.CleanScan()).Width(220).Height(44).Clicked() {
				a.startScan()
			}
		})
		return
	}

	selected, bytes := st.selection()
	var total int64
	for _, cand := range st.candidates {
		total += cand.Bytes
	}

	if !st.reviewing || len(st.candidates) == 0 {
		a.cleanHero(c, pal, 196, func() {
			Text(c, locales.CleanFound(humanBytes(total))).Font(fontMono).FontSize(26).FontWeight(500).
				TextColor(appTheme.Text).NoWrap()
			Row(c).Gap(6).AlignItems(Center).Children(func() {
				Text(c, locales.CleanCleanable(humanBytes(bytes))).FontSize(12).TextColor(quietText())
				Text(c, "·").FontSize(12).TextColor(quietText())
				rescan := ButtonBase(c).Label(locales.CleanRescan())
				rescan.Children(func() {
					Text(c, locales.CleanRescan()).FontSize(12).TextColor(Hex("#8fb4ff"))
				})
				if rescan.Clicked() {
					a.startScan()
				}
			})
			if len(st.candidates) > 0 && primaryPill(c, locales.CleanReview()).Width(220).Height(44).Margin(8, 0, 0, 0).Clicked() {
				st.reviewing = true
			}
			if st.finished != "" {
				Text(c, st.finished).FontSize(12).TextColor(appTheme.Success)
			}
		})
		if len(st.candidates) == 0 {
			Text(c, locales.CleanNothing()).FontSize(12.5).TextColor(appTheme.TextMuted).
				TextAlign(Center).FillWidth()
		}
		return
	}

	// The review: what was found, by category, and the one action.
	Row(c).FillWidth().Gap(10).AlignItems(Center).Padding(6, 0, 0, 0).Children(func() {
		if iconButton(c, iconBack, locales.CleanBack()).Clicked() {
			st.reviewing = false
		}
		planet(c, 96, pal)
		Column(c).Gap(3).Children(func() {
			Text(c, locales.CleanFound(humanBytes(total))).Font(fontMono).FontSize(sizeValue).FontWeight(500).
				TextColor(appTheme.Text).NoWrap()
			Text(c, locales.CleanSelected(selected, humanBytes(bytes))).FontSize(13.5).
				TextColor(fgSecondary())
		})
		Spacer(c)
		if st.finished != "" {
			Text(c, st.finished).FontSize(12).TextColor(appTheme.Success)
		}
		if ghostPill(c, locales.CleanRescan()).Clicked() {
			a.startScan()
		}
		if selected > 0 {
			label := locales.CleanDelete()
			if a.Settings.PermanentDelete {
				label = locales.CleanDeletePermanent()
			}
			if primaryPill(c, label).MinWidth(200).Height(44).Clicked() {
				a.deleteSelected()
			}
		}
	})

	if len(st.candidates) == 0 {
		cardWith(c, 4, func() {
			Text(c, locales.CleanNothing()).FontSize(13).TextColor(appTheme.TextMuted)
		})
		return
	}

	cardWith(c, 16, func() {
		Column(c).Gap(12).Children(func() {
			for _, category := range categoryOrder(st.candidates) {
				group := candidatesOf(st.candidates, category)
				var sum int64
				for _, cand := range group {
					sum += cand.Bytes
				}
				Column(c).Gap(1).Children(func() {
					Row(c).FillWidth().Gap(8).AlignItems(Center).Padding(4, 8, 2, 8).Children(func() {
						Box(c).Size(6, 6).Radius(3).Background(pal.glow)
						capsLabel(c, category, quietText())
						Spacer(c)
						Text(c, humanBytes(sum)).Font(fontMono).FontSize(sizeMicro).
							TextColor(quietText()).Width(84).TextAlign(End)
					})
					for j, cand := range group {
						a.candidateRow(c, cand)
						if j < len(group)-1 {
							rowDivider(c)
						}
					}
				})
			}
		})
	})
}

// cleanHero is the centered planet and the one action above the review: one
// stage, vertically placed in the window, with the globe over the text rather
// than a globe dropped at the top of a scroll page.
func (a *App) cleanHero(c *Context, pal palette, size float32, content func()) {
	Column(c).FillWidth().AlignItems(Center).
		Padding(16, 0, 16, 0).Gap(28).Children(func() {
		planet(c, size, pal)
		Column(c).FillWidth().AlignItems(Center).Gap(12).Children(content)
	})
}

func (a *App) candidateRow(c *Context, cand scan.Candidate) {
	row := listRow(c)
	row.Children(func() {
		if cand.Blocked != nil {
			Text(c, "—").Width(18).TextColor(quietText())
		} else {
			checked := a.clean.checked[cand.Path]
			// The third argument is the visible caption; the path belongs on
			// the row, not beside the box, so it goes on the accessible name.
			box := Checkbox(c, &checked, "").Label(cand.Path)
			if box.Changed() {
				a.clean.checked[cand.Path] = checked
			}
		}
		Column(c).Grow(1).Gap(2).Children(func() {
			Text(c, cand.Path).FontSize(12.5).TextColor(fgPrimary()).NoWrap()
			switch {
			case cand.Blocked != nil:
				Text(c, locales.CleanBlocked(cand.Blocked.Error())).FontSize(sizeCaption).TextColor(appTheme.Warning)
			case cand.Note != "":
				Text(c, cand.Note).FontSize(sizeCaption).TextColor(fgMuted())
			}
		})
		Text(c, humanBytes(cand.Bytes)).Font(fontMono).FontSize(sizeMeta).
			TextColor(fgSecondary()).Width(84).TextAlign(End).NoWrap()
	})
	if row.Hovered() {
		row.Background(faintFill())
	}
}

func (a *App) startScan() {
	if a.clean.scanning || a.Win == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.clean.scanning = true
	a.clean.err = ""
	a.clean.finished = ""
	a.clean.cancel = cancel
	go func() {
		progress := func(p scan.Progress) {
			a.Win.Update(func() { a.clean.progress = p })
		}
		found, err := a.Clean.Scan(ctx, progress)
		a.Win.Update(func() {
			a.clean.scanning = false
			a.clean.scanned = true
			a.clean.reviewing = false
			a.clean.cancel = nil
			a.clean.progress = scan.Progress{}
			if err != nil {
				a.clean.err = err.Error()
				return
			}
			a.clean.candidates = found
			a.clean.checked = map[string]bool{}
			for _, cand := range found {
				if cand.Blocked == nil && cand.Checked {
					a.clean.checked[cand.Path] = true
				}
			}
		})
	}()
}

// cleanConfirmFor is the confirmation for the mode the deletion will use: the
// Recycle Bin promises a restore, so a permanent delete must not reuse that
// wording or its button.
func cleanConfirmFor(permanent bool, size string) (detail, confirm string) {
	if permanent {
		return locales.CleanConfirmPermanentDetail(size), locales.CleanDeletePermanent()
	}
	return locales.CleanConfirmDetail(size), locales.CleanDelete()
}

func (a *App) deleteSelected() {
	chosen, bytes := a.clean.selectionCandidates()
	if len(chosen) == 0 || a.clean.busy || a.Win == nil {
		return
	}
	permanent := a.Settings.PermanentDelete
	detail, confirm := cleanConfirmFor(permanent, humanBytes(bytes))
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
		a.Win.Update(func() { a.clean.busy = true })
		results := a.Clean.Delete(context.Background(), chosen, permanent)
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
				delete(a.clean.checked, r.Path)
			}
			kept := a.clean.candidates[:0]
			for _, cand := range a.clean.candidates {
				if !removed[cand.Path] {
					kept = append(kept, cand)
				}
			}
			a.clean.candidates = kept
			a.clean.busy = false
			a.clean.finished = locales.CleanResult(ok, failed)
			a.reload()
		})
	}()
}

func (s *cleanState) selection() (count int, bytes int64) {
	for _, cand := range s.candidates {
		if s.checked[cand.Path] && cand.Blocked == nil {
			count++
			bytes += cand.Bytes
		}
	}
	return count, bytes
}

func (s *cleanState) selectionCandidates() ([]scan.Candidate, int64) {
	var chosen []scan.Candidate
	var bytes int64
	for _, cand := range s.candidates {
		if s.checked[cand.Path] && cand.Blocked == nil {
			chosen = append(chosen, cand)
			bytes += cand.Bytes
		}
	}
	return chosen, bytes
}

func categoryOrder(candidates []scan.Candidate) []string {
	var out []string
	seen := map[string]bool{}
	for _, c := range candidates {
		if !seen[c.Category] {
			seen[c.Category] = true
			out = append(out, c.Category)
		}
	}
	return out
}

func candidatesOf(candidates []scan.Candidate, category string) []scan.Candidate {
	var out []scan.Candidate
	for _, c := range candidates {
		if c.Category == category {
			out = append(out, c)
		}
	}
	return out
}

// humanBytes writes a byte count the way Windows does, with a decimal unit.
func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}
