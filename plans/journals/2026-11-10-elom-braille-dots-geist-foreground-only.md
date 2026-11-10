---
title: "Elom: Braille dot globes, Geist, and a foreground-only app"
date: 2026-11-10
summary: "Replaced the glyph-ramp planets with a smooth Braille-style dot matrix, embedded Geist/Geist Mono, reworked the nav, status, lists and review chrome around it, and removed the tray flyout and background lifetime."
---

# What happened

Ten UI/UX reviews ran in parallel over the rendered screens and `internal/ui`
(Gallery PNGs plus code), then the changes were integrated in one pass:

- **Planets** (`internal/ui/planet.go`): the character ramp is gone. The globe is a
  Braille-style dot matrix: one dot per surface sample at an even pitch, about 64 dots
  across a hero and 40 on a tile, snapped to whole pixels. Dot density carries tone via a
  white-noise threshold (`dotThreshold`); an ordered tile was tried first and read as a
  woven plaid at 2–3 DIP. Texture octaves were lowered so variation never exceeds one
  threshold step per cell, the land/water step widened, and the limb gets a gentle depth
  falloff. `appMark` uses the same renderer, so the nav mark is no longer a gradient bead.
- **Typography**: Geist and Geist Mono (SIL OFL, `internal/ui/fonts/`) are embedded and
  registered from `ui.New`; the stacks fall back to Segoe UI Variable and Cascadia Mono.
  Data speaks mono: `bigNumber`, status values, chips, sizes, timestamps, caps labels
  (`capsLabel`), nav tab labels and the Optimize couplet. The Georgia poem is gone.
- **Chrome**: the nav tray is a radius-12 panel over a dark wash instead of a glass pill;
  the chosen tab is the screen's ink (mid mixed 30% toward lit) with the base as text;
  sub-tabs and language segments use the same ink tint and square-ish segments; ghost
  pills use radius 8; toggles are 44×24 and carry accessible names.
- **Layout**: the clean hero is a full-height stage (centered, globe → 280 DIP horizon →
  promise → 44 DIP CTA); the promise is two balanced lines. Analyze gained an up button;
  treemap fills derive from Jupiter's ink (`ink[0]`→`ink[1]`, alternate tiles lifted
  toward lit) with contrast-picked labels, two-line bands and rune-safe fitting.
- **Status**: tiles fill the last row (no 276×164 hole), CPU is labelled and comes before
  MEM, the value column is mono, the process card height follows its rows, and meters and
  charts sit on a dotted field.
- **Lists**: apps and startup rows are keyboard-reachable (`ListState.Selected` + `Label`);
  the framework paints a chosen row in the theme accent (the cream primary fill), so rows
  paint an opaque card ground over it and the chosen row uses the screen ink. Letter
  monograms and process glyphs use the planet ink instead of six foreign hues.
- **Lifecycle** (`main.go`): tray icon, flyout panel, global shortcut and hide-on-close are
  removed. Closing the window quits; the sampler and `KeepAwake` release end with the app.
  `PanelView`, its helpers, four catalog messages and `resources/tray.png` are deleted.
- **Identity**: `mygo.json` identifier is `labs.hongducdev.elom`.

# Decisions

- Braille is a 2×4 dot matrix; drawing the dots directly is the same picture as the
  glyph without the font's line-box gaps, and it needs no font fallback. Geist Mono and
  Consolas do not carry U+2800, so the glyph route was also fragile.
- A white-noise threshold beats Bayer at small pitches; the hero and tile globes share one
  renderer with size-dependent dot counts.
- Primary actions stay the one cream capsule; secondary controls and selected rows are ink.
- No new dependency; insert only geometry/text/dot drawing.

# Evidence

- `go test ./... -count=1`: all packages pass; `go vet ./...` silent.
- `go tool mygo i18n check .`: 165 messages in en and vi.
- `go tool mygo build`: `Elom.exe` 12.6 MB and `elom-setup-0.1.0-amd64.exe` 4.1 MB.
- Launch/close smoke: `Start-Process` → `CloseMainWindow()` → process `HasExited` true
  (no tray keeps it alive).
- `TestGallery` renders `.gallery/clean-empty.png`, `clean-review.png`, `clean-list.png`,
  `software.png`, `startup.png`, `optimize.png`, `analyze.png`, `status.png`,
  `settings.png`; all reviewed after the final pass.
- `TestGeistFamiliesAreRegistered` pins the embedded families; ramp tests were replaced by
  `TestDotThresholdStaysInRange`.

# Next steps

- The 20 Hz dot globe's cost is measured only headless (≈56 ms per full 1180×760 frame in
  the software rasterizer); measure the GPU backend in the running app before tuning.
- Live pass for the wash cross-fade, reduce-motion and the clean review flow.
- `docs/verification.md` now records both gaps.

# Follow-up: an even field that still shows the planet

The first dot build inked a dot only when its brightness beat a noise threshold. That
carried tone, but it punched holes wherever the surface dipped, and a fractional dot pitch
rounded to alternating 3/4 DIP gaps. The globes looked mottled and uneven.

- Every dot inside the disc is drawn, at a whole-DIP pitch (`round(size/56)` for heroes,
  `round(size/30)` below 150 DIP, dot positions snapped), so there are no holes and no
  wobbling grid.
- Making every dot the same ink then erased the surface: the body read as a solid mesh.
  Three tiers now carry the features — dark (`ink[0]`), body (`ink[0]` mixed toward
  `ink[1]`), lit (`ink[2]`) — and the lit tier also draws a larger dot. The thresholds are
  per body (`planetKind.tiers`), so maria, basins, belts and spots stay readable.
- Textures were retuned to show features on any face and at any angle: scattered
  continents and small polar caps on Earth, rust seas on Mars, crater basins and rays on
  Mercury, broad maria on the Moon, strong latitude belts with a dark storm on Jupiter,
  granulation with two sunspots. Frequencies sit around 1-1.8 rad so a 30-dot tile still
  resolves several features.
- A low-alpha color underlay was tried under the dots and removed: the cell squares
  overshot the limb and turned the globe into a flat, boxy disc.
- Under 40 DIP the tiers are suppressed: a nav mark has no room for a lit tier and would
  collapse into a white blob.

Evidence: gallery re-rendered and reviewed for clean, clean-list, software/software-empty,
optimize, analyze, status and settings; `go test ./...`, `go vet ./...` and `i18n check`
pass. Headless cost is about 67 ms for a full 1180x760 clean frame (software rasterizer);
GPU cost remains unmeasured.
