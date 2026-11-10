package ui

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/egoist/mygo"
	. "github.com/egoist/mygo/ui"

	"elom/internal/core/metrics"
	"elom/internal/core/scan"
	"elom/locales"
)

// analyzeState is the Analyze screen's state.
type analyzeState struct {
	root    *scan.Node
	current *scan.Node
	stack   []*scan.Node
	largest []*scan.Node
	table   ListState
	picked  int
	chosen  map[string]bool

	scanning bool
	busy     bool
	progress scan.IndexProgress
	err      string
	cancel   context.CancelFunc

	// view is the right side's tab: the map, or the largest files.
	view int
	// driveUsed and driveTotal describe the volume of the scanned folder.
	driveUsed, driveTotal uint64

	hover int
	tiles []Tile
	// tiles belong to tilesNode at tilesRect; the layout is rebuilt when
	// either changes.
	tilesNode *scan.Node
	tilesRect Rect
}

func newAnalyzeState() analyzeState {
	return analyzeState{picked: -1, hover: -1, chosen: map[string]bool{}}
}

func (a *App) analyzePage(c *Context, pal palette) {
	st := &a.analyze

	if st.err != "" {
		cardWith(c, 4, func() {
			Text(c, st.err).FontSize(12.5).TextColor(appTheme.TextMuted)
		})
	}

	switch {
	case st.scanning:
		cardWith(c, 8, func() {
			Row(c).Gap(10).AlignItems(Center).Children(func() {
				Spinner(c)
				Text(c, locales.AnalyzeScanning(st.progress.Entries)).FontSize(12.5).TextColor(appTheme.TextMuted)
				Spacer(c)
				if cancel := st.cancel; cancel != nil {
					if ghostPill(c, locales.CleanCancel()).Clicked() {
						cancel()
					}
				}
			})
		})
		return
	case st.root == nil:
		Column(c).FillWidth().AlignItems(Center).Padding(28, 0, 20, 0).Gap(14).Children(func() {
			planet(c, 150, pal)
			Text(c, locales.AnalyzeEmpty()).FontSize(13).TextColor(appTheme.TextMuted).
				MaxWidth(440).TextAlign(Center)
			Row(c).Gap(10).Children(func() {
				if primaryPill(c, locales.AnalyzeStart()).Clicked() {
					a.startIndex(homeDir())
				}
				if ghostPill(c, locales.AnalyzeChoose()).Clicked() {
					a.chooseFolder()
				}
			})
		})
		return
	}

	if st.busy {
		Row(c).Gap(10).AlignItems(Center).Children(func() {
			Spinner(c)
			Text(c, locales.CleanDelete()).FontSize(12.5).TextColor(appTheme.TextMuted)
		})
	}

	current := st.current
	Row(c).FillWidth().Grow(1).Gap(18).AlignItems(Stretch).Children(func() {
		// The side: the planet for the folder, its size, and its children.
		Column(c).Width(240).Gap(10).Children(func() {
			Column(c).FillWidth().AlignItems(Center).Gap(10).Padding(6, 0, 4, 0).Children(func() {
				planet(c, 92, pal)
				Text(c, locales.AnalyzeItems(current.Files, humanBytes(current.Bytes))).
					FontSize(11.5).Font("Consolas").TextColor(softText()).NoWrap()
			})
			Text(c, locales.AnalyzeCurrent()).FontSize(11.5).FontWeight(600).TextColor(softText()).Padding(4, 4, 0, 4)
			List(c, nil, len(current.Children), func(i int) {
				child := current.Children[i]
				row := listRow(c)
				row.Children(func() {
					icon := iconFile
					if child.Dir {
						icon = iconFolder
					}
					Icon(c, icon).FontSize(14).TextColor(fgMuted())
					Column(c).Grow(1).Gap(2).Children(func() {
						Text(c, trim(child.Name, 28)).FontSize(12.5).TextColor(fgPrimary()).NoWrap()
						Text(c, humanBytes(child.Bytes)).Font(fontMono).FontSize(10.5).
							TextColor(fgMuted()).NoWrap()
					})
					if child.Dir {
						Icon(c, iconChevron).FontSize(12).TextColor(fgFaint())
					}
				})
				if row.Hovered() || i == st.hover {
					row.Background(faintFill())
				}
				row.OnClick(func() { a.openChild(i) })
				if i < len(current.Children)-1 {
					rowDivider(c)
				}
			}).FillWidth().Grow(1)
			hintText(c, locales.AnalyzeHint())
		})

		// The map, under a breadcrumb and the drive's use.
		Column(c).Grow(1).Gap(10).Children(func() {
			Row(c).FillWidth().Gap(6).AlignItems(Center).Children(func() {
				a.breadcrumb(c)
				Spacer(c)
				if st.driveTotal > 0 {
					bar(c, float32(float64(st.driveUsed)/float64(st.driveTotal)), pal.glow, 4).Width(54)
					Text(c, locales.AnalyzeDriveUsed(humanBytes(int64(st.driveUsed)), humanBytes(int64(st.driveTotal)))).
						FontSize(11).TextColor(quietText()).NoWrap()
				}
				views := []string{locales.AnalyzeMap(), locales.AnalyzeLargest()}
				seg := SegmentedBase(c, &st.view, len(views))
				seg.Track.Padding(2).Margin(0, 0, 0, 8).Radius(999).Background(RGBA(255, 255, 255, 0.06)).Children(func() {
					for i, label := range views {
						s := seg.Segment(i).Padding(3, 10).Radius(999).Label(label)
						color := fgSecondary()
						if i == st.view {
							s.Background(Hex("#f7f5f0"))
							color = Hex("#12161f")
						}
						s.Children(func() { Text(c, label).FontSize(11).NoWrap().TextColor(color) })
					}
				})
				if iconButton(c, iconFolder, locales.AnalyzeChoose()).Clicked() {
					a.chooseFolder()
				}
				if iconButton(c, iconRefresh, locales.AnalyzeRescan()).Clicked() {
					a.startIndex(current.Path)
				}
			})
			if st.view == 1 {
				a.analyzeLargest(c, pal)
				return
			}
			mapBox := Box(c).FillWidth().Grow(1).MinHeight(240).Radius(10).
				Draw(func(p *Painter, r Rect) {
					a.paintTreemap(p, r, pal)
				}).
				HitTest(func(x, y float32, r Rect) bool { return tileAt(a.analyze.tiles, x, y) >= 0 })

			if mapBox.Hovered() {
				if x, y, ok := mapBox.PointerPosition(); ok {
					st.hover = tileAt(st.tiles, x, y)
				}
			} else {
				st.hover = -1
			}
			if mapBox.Clicked() {
				if x, y, ok := mapBox.PointerPosition(); ok {
					if i := tileAt(st.tiles, x, y); i >= 0 {
						a.openChild(i)
					}
				}
			}
		})
	})
}

// breadcrumb names the folders from the scanned root to the current one; a
// click goes back to any of them.
func (a *App) breadcrumb(c *Context) {
	st := &a.analyze
	trail := append(append([]*scan.Node(nil), st.stack...), st.current)
	for i, node := range trail {
		if i > 0 {
			Icon(c, iconChevron).FontSize(11).TextColor(quietText())
		}
		last := i == len(trail)-1
		b := ButtonBase(c).Padding(3, 4).Radius(6).Label(node.Name)
		b.Children(func() {
			color := softText()
			if last {
				color = appTheme.Text
			}
			t := Text(c, trim(node.Name, 26)).FontSize(12).NoWrap().TextColor(color)
			if last {
				t.FontWeight(600)
			}
		})
		if !last && b.Clicked() {
			st.current = node
			st.stack = st.stack[:i]
			a.resetSelected()
		}
	}
}

func (a *App) paintTreemap(p *Painter, r Rect, pal palette) {
	st := &a.analyze
	if st.current == nil || len(st.current.Children) == 0 {
		return
	}
	inner := Rect{X: r.X + 4, Y: r.Y + 4, W: r.W - 8, H: r.H - 8}
	if st.tilesNode != st.current || st.tilesRect != inner {
		values := make([]float64, len(st.current.Children))
		for i, child := range st.current.Children {
			values[i] = float64(child.Bytes)
		}
		st.tiles = squarify(values, inner)
		st.tilesNode = st.current
		st.tilesRect = inner
	}
	for _, tile := range st.tiles {
		child := st.current.Children[tile.Index]
		fill := pal.ink[1]
		if len(pal.tiles) > 0 {
			fill = pal.tiles[tile.Index%len(pal.tiles)]
		}
		if tile.Index == st.hover {
			fill = fill.Mix(Hex("#ffffff"), 0.18)
		}
		rect := Rect{X: tile.Rect.X + 1.5, Y: tile.Rect.Y + 1.5, W: tile.Rect.W - 3, H: tile.Rect.H - 3}
		if rect.W <= 0 || rect.H <= 0 {
			continue
		}
		if tile.Index == st.hover {
			p.Stroke(rect, RGBA(255, 255, 255, 0.28), radiusTile, 1)
		}
		p.FillGradient(rect, LinearGradient{
			Angle: 90,
			From:  fill.Mix(Hex("#ffffff"), 0.10),
			To:    fill,
		}, radiusTile)
		ink := readableOn(fill)
		glyph := iconFile
		if child.Dir {
			glyph = iconFolder
		}
		pad := float32(8)
		nameSize := float32(12.5)
		sizeText := humanBytes(child.Bytes)
		labelFits := rect.W > 70 && rect.H > 30
		switch {
		case labelFits:
			// Top-left, as Mole's treemap cells: icon, name, size.
			p.Icon(glyph, Rect{X: rect.X + pad, Y: rect.Y + pad + 1, W: 12, H: 12}, ink.Alpha(0.85))
			avail := rect.W - pad*2 - 18
			name := child.Name
			if w := measureTextWidth(p, name, nameSize); w > avail {
				n := int(float64(len(name)) * float64(avail) / float64(w))
				if n < 1 {
					n = 1
				}
				name = name[:n]
			}
			p.Text(rect.X+pad+18, rect.Y+pad-2, name, nameSize, ink)
			if rect.H > 44 {
				p.RichText(rect.X+pad+18, rect.Y+pad+16, 0, Span{
					Text:  sizeText,
					Font:  fontMono,
					Size:  10,
					Color: ink.Alpha(0.72),
				})
			}
		case rect.W > 22 && rect.H > 22:
			p.Icon(glyph, Rect{X: rect.X + rect.W/2 - 6, Y: rect.Y + rect.H/2 - 6, W: 12, H: 12}, ink.Alpha(0.7))
		}
	}
}

func (a *App) analyzeLargest(c *Context, pal palette) {
	st := &a.analyze
	if len(st.largest) == 0 {
		return
	}
	st.table.Selected = &st.picked
	chosen := a.chosenPaths()
	var chosenBytes int64
	for _, path := range chosen {
		for _, file := range st.largest {
			if file.Path == path {
				chosenBytes += file.Bytes
			}
		}
	}
	cardWith(c, 12, func() {
		Row(c).FillWidth().Gap(10).AlignItems(Center).Children(func() {
			Box(c).Size(6, 6).Radius(3).Background(pal.glow)
			Text(c, locales.AnalyzeLargest()).FontSize(10.5).FontWeight(600).
				LetterSpacing(1.0).TextColor(quietText()).NoWrap()
			if len(chosen) > 0 {
				Text(c, locales.CleanSelected(len(chosen), humanBytes(chosenBytes))).FontSize(11).TextColor(softText())
			}
			Spacer(c)
			if len(chosen) == 0 && st.picked >= 0 && st.picked < len(st.largest) {
				if ghostPill(c, locales.AnalyzeReveal()).Clicked() {
					mygo.Shell.ShowItemInFolder(st.largest[st.picked].Path)
				}
			}
			if len(chosen) > 0 {
				if primaryPill(c, locales.AnalyzeTrash()).Clicked() {
					a.trashPaths(chosen)
				}
			}
		})
		Table(c, &st.table, []TableColumn{
			{Title: "", Width: 28, Fixed: true},
			{Title: "NAME", Width: 0, Fixed: true},
			{Title: "SIZE", Width: 110, Align: End, Fixed: true},
			{Title: "MODIFIED", Width: 170, Fixed: true},
		}, len(st.largest), func(row, col int) {
			file := st.largest[row]
			switch col {
			case 0:
				checked := st.chosen[file.Path]
				box := Checkbox(c, &checked, "")
				if box.Changed() {
					st.chosen[file.Path] = checked
				}
			case 1:
				Text(c, file.Name).FontSize(12.5).NoWrap()
			case 2:
				Text(c, humanBytes(file.Bytes)).Font(fontMono).FontSize(11.5).NoWrap()
			case 3:
				when := "—"
				if !file.Modified.IsZero() {
					when = file.Modified.Format("2006-01-02 15:04")
				}
				Text(c, when).Font(fontMono).FontSize(11).TextColor(fgMuted()).NoWrap()
			}
		}).Height(240)
	})
}

func (a *App) openChild(i int) {
	st := &a.analyze
	if st.current == nil || i < 0 || i >= len(st.current.Children) {
		return
	}
	child := st.current.Children[i]
	if !child.Dir {
		for j, file := range st.largest {
			if file.Path == child.Path {
				st.picked = j
				return
			}
		}
		return
	}
	st.stack = append(st.stack, st.current)
	st.current = child
	a.resetSelected()
}

func (a *App) resetSelected() {
	st := &a.analyze
	st.picked = -1
	st.hover = -1
	st.largest = scan.LargestFiles(st.current, 100)
	st.chosen = map[string]bool{}
	st.tiles = nil
	st.tilesNode = nil
	st.tilesRect = Rect{}
}

func (a *App) startIndex(root string) {
	if a.analyze.scanning || a.Win == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	st := &a.analyze
	st.scanning = true
	st.err = ""
	st.cancel = cancel
	st.progress = scan.IndexProgress{}
	limits := scan.Limits{MaxDepth: a.Settings.MaxScanDepth, MaxEntries: a.Settings.MaxScanEntries}
	go func() {
		progress := func(p scan.IndexProgress) {
			a.Win.Update(func() { a.analyze.progress = p })
		}
		node, err := scan.Index(ctx, root, limits, progress)
		used, total, _ := metrics.DriveUsage(root)
		a.Win.Update(func() {
			st.driveUsed, st.driveTotal = used, total
			st.scanning = false
			st.cancel = nil
			if err != nil && node == nil {
				st.err = locales.AnalyzeError(root, err.Error())
				return
			}
			if errors.Is(err, scan.ErrLimit) {
				st.err = locales.AnalyzePartial()
			} else {
				st.err = ""
			}
			st.root = node
			st.current = node
			st.stack = nil
			a.resetSelected()
		})
	}()
}

func (a *App) chooseFolder() {
	if a.Win == nil {
		return
	}
	go func() {
		dir, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.Win,
			Title:     locales.AnalyzeChoose(),
			Directory: true,
		})
		if err != nil || len(dir) == 0 {
			return
		}
		a.Win.Update(func() { a.startIndex(dir[0]) })
	}()
}

// chosenPaths returns the rows checked in the largest-files table.
func (a *App) chosenPaths() []string {
	var paths []string
	for path, on := range a.analyze.chosen {
		if on {
			paths = append(paths, path)
		}
	}
	return paths
}

func (a *App) trashPaths(paths []string) {
	if len(paths) == 0 {
		return
	}
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             a.Win,
			Type:               mygo.MessageWarning,
			Message:            locales.AnalyzeConfirm(len(paths)),
			Detail:             strings.Join(paths, "\n"),
			Buttons:            []string{locales.CleanCancel(), locales.AnalyzeTrash()},
			DefaultButton:      1,
			CancelButton:       0,
			DestructiveButtons: []int{1},
		})
		if err != nil || res.Button != 1 {
			return
		}
		a.Win.Update(func() { a.analyze.busy = true })
		results := a.Clean.DeletePaths(context.Background(), paths)
		a.Win.Update(func() {
			a.analyze.busy = false
			ok := 0
			for _, r := range results {
				if r.Err == "" {
					ok++
				}
			}
			if ok > 0 {
				a.reload()
				a.startIndex(a.analyze.current.Path)
			}
		})
	}()
}

// tileAt returns the tile under a point in the treemap's coordinates.
func tileAt(tiles []Tile, x, y float32) int {
	for i := range tiles {
		if tiles[i].Rect.Contains(x, y) {
			return tiles[i].Index
		}
	}
	return -1
}

// trim shortens s to at most n runes for a label that may not fit.
func trim(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n < 2 {
		return string(runes[:n])
	}
	return string(runes[:n-1]) + "…"
}

func homeDir() string {
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home
	}
	home, _ := os.UserHomeDir()
	return home
}

// measureTextWidth is how wide s is in the painter's body font, for labels
// that must be cut to fit a treemap cell.
func measureTextWidth(p *Painter, s string, size float32) float32 {
	w, _ := p.MeasureText(0, Span{Text: s, Size: size})
	return w
}
