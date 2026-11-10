---
phase: 4
title: "Lists and surfaces"
status: done
effort: "2.5d"
depends: [1, 2]
---

# Phase 4 — Lists and surfaces

Goal: one row vocabulary across Apps, Startup, Analyze and Settings, plus Mole's treemap
label layout and an overlay for detail. Behaviour (what is scanned, selected, deleted) does
not change.

Reference: `AppRowView.cs` (row + expand), `SoftwarePage.cs`, `TreemapCanvas.cs`,
`SettingsOverlay.cs`, `DetailsOverlay.cs`, shots `03-apps-uninstall.png`, `07-analyze.png`,
`12-settings-general.png`.

Current state (verified in code): `apps.go` already has a segmented control, search, sort
links, app icons, monospace facts and a check mark; `analyze.go` treemap already has 3 px
gaps, radius 4 and hover lightening; `clean.go` already groups by category with counts;
`startup.go` rows have no hairline; `shell.go` settings is five stacked cards. So this phase
is alignment, not a rewrite.

## Files

| File | Change |
|---|---|
| `internal/ui/apps.go` | segment style, list card, row primitive, expandable detail |
| `internal/ui/software.go` | detail content moves into the expanded row; leftover card styled |
| `internal/ui/startup.go` | rows on the shared primitive with dividers |
| `internal/ui/analyze.go` | treemap labels/fill/hover ring, sidebar rows, largest-files cells |
| `internal/ui/clean.go` | review rows on the primitive, sticky action bar, hero numbers |
| `internal/ui/shell.go` | settings rebuild as one card of rows |
| `internal/ui/design.go` | `detailOverlay` helper (if used) |
| `internal/ui/*_test.go` | update only for intentional tree/format changes |
| `locales/en.json`, `locales/vi.json`, `locales/catalog_gen.go` | new keys, then `go tool mygo i18n` |

## Tasks

1. **Apps — segments and header**: active segment is the white pill with dark text (as the
   nav), inactive 0.7 white; keep `SegmentedBase`. Put the refresh button at the trailing
   edge. Wrap the list in `cardWith`: a head row (label + `count · size` in mono, 10 caps)
   then `rule`, then the rows.
2. **Apps — rows**: use `listRow` (44 px, 8 radius, hover 5% white) with icon 28, name 13
   medium, sub 11 muted (`publisher · version`), size mono 11 right, check mark 16 at the
   end, and a chevron when the row can expand. `rowDivider` between rows, inset 56.
3. **Apps — expand**: clicking the chevron (or the row) expands in place, as Mole's
   `ExpandContent`: install location, uninstall string and publisher in label/value rows,
   plus "Show in folder" and the uninstall button. Replace the separate detail card at the
   bottom with this expansion; keep the existing `st.picked` semantics so tests and the
   uninstall flow are unchanged.
4. **Startup rows**: same primitive, with icon from the command's executable when
   `mygo.Shell.FileIcon` returns one (cache per path), name 13, command mono 11 muted, source
   and state right, toggle at the end; dividers.
5. **Analyze — treemap** (`paintTreemap`):
   - fill: vertical gradient from `fill.Mix(white, 0.10)` to `fill`, radius 4;
   - label at top-left with 10 px inset: 12 px icon, name 12.5 medium, size 10 mono at 75%
     ink, single line with `MeasureText` truncation (drop name before size);
   - hover: 1 px white 12% stroke; chosen: 1.5 px accent stroke;
   - keep the squarify geometry and 3 px gaps.
6. **Analyze — sidebar and largest files**: sidebar rows on the primitive with icon, name,
   mono size and chevron; largest-files table uses mono size (right), mono muted modified
   time sentinel `—` when zero.
7. **Clean — review**: category header (10 caps + count + sum mono), rows 44 px with check
   mark, path 12.5, reason 11 muted, size mono 11 right, blocked chip; the action row
   (delete / rescan / back) is built once and pinned above the scrolling list. Hero number
   uses the display font at 30/32.
8. **Settings**: one card, max width 820, sections separated by 10 pt caps labels; each row
   is `label 12.5 + description 11 muted` left and control right, 44 px minimum, hairline
   between rows. Preserve every current control and error line (permanent delete, whitelist
   editor, log path/status, language segmented, clean screen, recent log in mono).
9. **Overlay** (only if step 3's inline expansion feels cramped in the 780 px pass): build
   `detailOverlay(c, width, build)` with `ui.Overlay` + `Absolute` centring, backdrop
   `rgba(0,0,0,0.45)`, panel radius 12, padding 18, `OverlayShortcut(0, KeyEscape)` to close,
   focus restored by MyGo. Use it for app details only; do not convert the clean review.
10. **i18n**: add keys for any new label (publisher, install location, uninstall command,
    show in folder) to `locales/en.json` and `locales/vi.json`, then regenerate.

## Acceptance

- Apps/Startup/Analyze sidebar/Settings rows use one primitive: same height, hover, divider
  and type sizes; no screen repeats its own row code.
- Apps rows expand to show the uninstall facts; the uninstall and leftover flows behave
  exactly as before (same state fields, same confirmations).
- Treemap labels render top-left, truncate instead of overflowing, and stay legible on every
  tile colour; the hover and selection rings are visible.
- Settings fits one card at 1216×780 without horizontal scroll; all controls still work.
- `.gallery/software.png`, `startup.png`, `analyze.png`, `clean-review.png`, `settings.png`
  reviewed against the reference shots.
- `go test ./...` green; only intentionally changed expectations were updated.

## Verification

```powershell
go test ./internal/ui -count=1
go tool mygo i18n check .
go vet ./...
go test ./internal/ui -run TestGallery -count=1
go tool mygo dev   # expand a row, uninstall dry-run, toggle startup, drill the treemap
```
