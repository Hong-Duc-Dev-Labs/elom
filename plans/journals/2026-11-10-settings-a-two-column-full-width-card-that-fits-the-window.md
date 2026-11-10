---
title: "Settings: a two-column full-width card that fits the window"
date: 2026-11-10
summary: Split Settings into two columns on a 1100 DIP card so Recent operations and the version footer are visible without scrolling at 1180x760
---

# Settings: a two-column full-width card that fits the window

# What happened

The Settings page was one 860 DIP card of five stacked sections. At the default
1180x760 window the card ran past the bottom edge: the last section header,
`RECENT OPERATIONS`, was cut mid-glyph and its rows sat below the fold. The rest
of the window width was empty beside the card.

`internal/ui/shell.go` now draws Settings on the screen's full 1100 DIP width as
two columns of sections inside one card:

- left: Preferences, Protection list, Operation log, Clean screen;
- right: Protected folders, Recent operations.

Both columns take 50 % of the row and shrink by half the 28 DIP gap
(`BasisPercent(50).Shrink(1)`). The whitelist editor grows to the column width
instead of its old fixed 460 DIP. Every unbounded mono line in the card (log
path, log record, whitelist pattern, allow root) is `SingleLine` with `…` and a
tooltip carrying the full value, so long paths cannot widen a column.

# Evidence

- `TestSettingsFitsTheDefaultWindow` renders the screen at 1180x760 (the
  configured window) and requires `RECENT OPERATIONS`, `Nothing recorded yet.`
  and the version label to end above 760 DIP.
- Headless renders at 1180x760 with 2 patterns and 12 log records: version at
  633 DIP, both columns 519.4 DIP wide; Vietnamese (forced through
  `i18n.SetLocaleFunc`) keeps the same geometry.
- `go test ./... -count=1`, `go vet ./...` and
  `go tool mygo i18n check .` (165 messages, en/vi) pass; `.gallery/settings.png`
  regenerated and reviewed.
- `TestSettingsLongPatternKeepsTheColumns` and
  `TestSettingsLongProtectedRootKeepsTheColumns`: equal 519.4 DIP columns, the
  right header inside the page, and Add/Remove on screen for a 150-character
  pattern and a 112-character profile.

# Decision

The page keeps MyGo's scroll container for short windows, but no longer needs it
at the default size. A single column could not fit: its measured content was
about 830 DIP tall against the 751 DIP client the default window gives the
view. Two narrower columns reuse the empty right half rather than shrinking the
type or dropping a control.

# Next steps

- A long whitelist (checked with 8 patterns headless) makes the page taller than
  the window again, so it scrolls: the whitelist list has no cap of its own.
- Live click-through of the gear tab was not automated; the view was verified by
  MyGo's headless renderer (`ui.NewTester`) and the gallery PNG.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.

# Follow-up: long paths truncate instead of moving the split

Review found the first version only capped the log lines. A whitelist pattern
was still `NoWrap().Grow(1)`: a 150-character pattern widened the left column to
1123 DIP, pushed Protected folders to x=1390 (outside the 1180 DIP window), and
put Add/Remove at x=1302/1327 — the user could no longer remove the pattern. A
76-character Chrome cache path already moved the split from 519/519 to
561/478. The allow roots had the same problem in the right column.

Fix: the pattern keeps `Grow(1).Shrink(1).MinWidth(0)` and, like the roots, is
`SingleLine().Ellipsis("…").Tooltip(full)`. Both new tests pin the invariant;
with the fix all four measured columns are 519.4 DIP and Remove sits at x=540.
