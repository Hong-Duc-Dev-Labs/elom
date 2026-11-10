---
phase: 3
title: "Disk Analyze"
status: pending
priority: P1
effort: "2d"
dependencies: [1, 2]
---

# Phase 3: Disk Analyze

## Goal

Turn a directory into a navigable treemap: scan and size a tree, lay it out as rectangles drawn by MyGo's painter, drill down with the mouse, list the largest files, and send a selection to the Recycle Bin.

## Context Links

- Scanner primitives: [phase-02-clean-engine.md](./phase-02-clean-engine.md)
- MyGo drawing and hit testing: https://mygo.egoist.dev/docs/ui/drawing.md
- Mole's disk map and its limits: https://mole.fit/windows/blog/windows-disk-analyzer-comparison

## Key Insights

- `Element.Draw` receives a `ui.Painter` and a rect in DIPs; a treemap is one element, one paint callback, and manual hit testing via `HitTest` — no per-cell widgets, no layout cost per file.
- A cell is only clickable where it paints, so `HitTest` must use the same geometry function as `Draw`, shared and unit-tested.
- Allocated size and logical size differ (compressed/sparse files); Windows Explorer shows both. Use logical length from `DirEntry.Info()` and say so in the UI, rather than spending a syscall per file.
- Hard links are counted once per path; `C:\Windows` and package stores over-report. Document; do not silently "fix" with a slow per-file handle open.

## Requirements

**Functional**

- Pick a drive or start at the user profile; breadcrumb path bar; `Up` and back navigation.
- Scan in a background goroutine with live counters (files, bytes, current directory); cancel.
- Treemap of the current directory: area proportional to bytes, squarified layout, folder color by depth/index, label when the cell is large enough.
- Hover shows name + size; click drills in; clicking a file opens the largest-files row for that file.
- Largest files table for the current subtree, top 200, sortable by size/date.
- Multi-select and "Move to Recycle Bin" with confirmation and oplog; blocked paths are not selectable.

**Non-functional**

- `C:\Users\<user>` indexed in ≤ 15 s on warm cache.
- Frame time stays under 16 ms while panning a 5 000-cell treemap; layout only recomputed when the node or size changes.
- Index memory ≤ ~100 bytes per file; children built lazily per directory.

## Architecture

```
scan.Index ──► Node{Path, Name, Bytes, Files, Children []Node, Err}
                  │
      ui.treemapLayout(items, rect) []Cell   // squarified, pure, tested
                  │
      ui.Box().Fill().Draw(paintCells).HitTest(hitCells)
                  │
      click ──► app.stack = append(app.stack, node.Path)
```

Node size accounting: a directory's bytes are the sum of descendant file bytes; a node whose `Err != nil` is drawn hatched and excluded from deletion. Index is built with `filepath.WalkDir` starting at the chosen root, capped by a depth limit (default 12) and an item cap (default 1 000 000 entries) so a pathological tree cannot exhaust memory.

Squarified layout signature (kept independent of MyGo so it is testable without a window):

```go
// Cell is a rectangle for one item, in the coordinate space of layout.
type Item struct{ Bytes int64; Key int }
func Layout(items []Item, w, h float64) []Cell
```

`Layout` sorts descending, takes the worst-aspect-ratio row greedily (squarified, Bruls et al.), and returns cells whose areas sum to `w*h` within 0.1%.

## Related Code Files

Create:

- `internal/core/scan/index.go`, `index_test.go` — tree build, caps, cancel
- `internal/ui/treemap.go`, `treemap_test.go` — layout + paint + hit test
- `internal/ui/analyze.go` — breadcrumb, treemap element, largest files table, actions
- `internal/svc/analyze.go` — scan job, progress, results

Modify:

- `main.go` — mount Analyze
- `locales/en.json` — Analyze strings

## Implementation Steps

1. `scan.Index` builder: `Index(root string, limits Limits, progress func(Progress), ctx context.Context) (*Node, error)`; skip reparse points; record `Err` per directory instead of aborting; track totals.
2. Tests on a synthetic tree: sizes sum correctly, depth and entry caps respected, unreadable directory produces `Err` and does not fail the scan, cancel returns partial index.
3. `ui.treemapLayout`: squarified algorithm, padding 1 DIP between cells, minimum cell size 3×3 DIP folded into the parent, deterministic ordering.
4. Tests: total area conservation, no overlapping cells, single item fills the rect, 10 000 items terminate under 10 ms, degenerate rects (zero width/height) return nil.
5. Analyze view: breadcrumb from the node stack, treemap element `Fill().Draw(...)`, hover tooltip with name and size, `HitTest` clicking into a directory or selecting a file, `Up`/Backspace navigation, "Scan another folder…" via `mygo.Dialog.Open` with `Directory: true`.
6. Largest-files table: top 200 files of the current subtree via `ui.Table`, sortable columns (size, modified), row action "Show in Explorer" (`mygo.Shell.ShowItemInFolder`) and "Move to Recycle Bin".
7. Selection model: clicking with Ctrl/Cmd extends the selection, `Esc` clears; status bar shows selected bytes; deletion runs `delete.Trash` with the same guard and oplog path as Clean.
8. Progress UI: counters while scanning, cancel button; on completion, show scan duration and entry count so the user can judge cost.
9. Wire the root choice: drives enumerated from `windows.GetLogicalDrives`, profile default.

## Todo

- [x] `scan.Index` with caps, per-directory errors, cancel
- [x] Index unit tests on a synthetic tree
- [x] Squarified layout function + tests (area, overlap, perf)
- [x] Treemap element: paint, hover highlight, hit test, drill-down
- [x] Path bar with Up navigation and folder picker
- [x] Largest-files table with reveal and Recycle Bin actions
- [x] Multi-select + Recycle Bin deletion through guard + oplog
- [x] Progress and cancel UI

## Evidence

Measured on this Windows 11 machine, catalog of 300k entry cap (the default):

| Run | Entries | Bytes | Time |
|---|---|---|---|
| `C:\Users\hongducdev`, cap 300 000 | 261 408 | 40.5 GB | **3.5 s** |
| `C:\Users\hongducdev`, cap 1 000 000 | 880 351 | 106.4 GB | 20.3 s (stopped at the cap) |
| Same, sequential walk before the optimization | 1 000 000 | — | 86.4 s (stopped at the cap) |

The walk parallelizes the first two levels (up to 8 × 8 subtrees) and stats
files only; that took ~880k entries from 86 s to 20 s. The default cap keeps
the first open of a developer profile under 4 s, and the screen says the sizes
are partial when it stops.

Automated (`go test ./...`):

- Squarified layout: areas sum to the rectangle within 0.1%, tiles stay inside,
  zero values produce no tile, 500 items never produce a negative rectangle.
- Index: sums and ordering on a synthetic tree, depth and entry caps, missing
  root recorded with its error, cancel returns a partial index.
- Drill-down/Up and selection state covered by headless view tests.
- Largest files returns the k biggest files of the subtree.

Needs a manual pass:

- Clicking tiles in a real window (hit testing is geometry-tested and shares the
  paint geometry, but no test clicks the map).
- Moving a selected file to the Recycle Bin and restoring it (`Shell.TrashItem`
  needs the running app; tests use a fake trasher).

## Success Criteria

- [ ] `C:\Users\<user>` scans in ≤ 15 s with the UI responsive and a working cancel.
- [ ] The treemap's largest cell corresponds to the largest child on disk (cross-checked in PowerShell).
- [ ] Drilling five levels deep works with the keyboard and the mouse.
- [ ] Deleting a selection moves it to the Recycle Bin, restores from Explorer, and writes one oplog record per path.
- [ ] A protected path (e.g. `C:\Windows`) cannot be selected for deletion and explains why.
- [ ] Frame stays smooth with 5 000 cells (no visible stutter when hovering across the map).

## Verification

```powershell
go test ./internal/ui -run TestLayout -v
go test ./internal/core/scan/...
go tool mygo dev
# ground truth for one folder:
Get-ChildItem "C:\Users" -Directory | ForEach-Object {
  [pscustomobject]@{ Name = $_.Name; Bytes = (Get-ChildItem $_.FullName -Recurse -File -ErrorAction SilentlyContinue |
    Measure-Object Length -Sum).Sum }
} | Sort-Object Bytes -Descending
```

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Scanning a huge drive blocks memory | Depth and entry caps; partial index state shown in the UI |
| Logical vs allocated size confuses users | Label the metric ("logical size") in the UI and in the README |
| Hard-link double counting | Documented in the Analyze screen's info tooltip |
| Hit testing disagrees with painted cells | One geometry source (`layout` result) used by both paint and hit test; unit test compares cell containment |
| Treemap of thousands of cells is slow to paint | Paint fills only visible cells; cells above the viewport clip are skipped; measure with the frame counter from the inspector |

## Security Considerations

- The treemap never deletes without an explicit confirmation dialog listing paths and bytes.
- Guard and oplog are the same as phase 2; Analyze adds no new deletion path.
- Scanning enumerates file names only and stores them in memory; nothing is persisted.

## Next Steps

Phase 4 adds the sampler and the tray panel, reusing the shell's sidebar and window plumbing.
