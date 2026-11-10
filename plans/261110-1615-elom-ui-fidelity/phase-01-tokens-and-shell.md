---
phase: 1
title: "Tokens, shell and primitives"
status: done
effort: "1d"
depends: []
---

# Phase 1 — Tokens, shell and primitives

Goal: one token set and four shared primitives drive every screen, so later phases only
change content. No screen layout changes in this phase beyond the shell wash and nav.

Reference: Mole `Surface.cs` (base/accent), `TabWash.cs` (alpha ramp, composite, sink, lift),
`TabWashes.cs` (per-tab colour), `HudTile.cs` (card metrics), `LanguageFonts.cs` (faces).

## Files

| File | Change |
|---|---|
| `internal/ui/design.go` | tokens, wash math, card/row/segment/rail, type helpers, fonts, palette rewrite |
| `internal/ui/shell.go` | `View` background wash + cross-fade, nav metrics |
| `internal/ui/design_test.go` | **new**: wash math vectors, contrast checks |
| `internal/ui/shell_test.go` | update only if the nav tree changed |

## Tasks

1. **Token block** in `design.go` (above `palette`):
   - `const baseBG = 0x15141b`, `const accentMint = 0x4fd6a9`.
   - Text tones as functions of the base, not white: `fgPrimary #ededee` (≈0.93),
     `fgSecondary 0.72`, `fgMuted 0.55`, `fgFaint 0.35`, `hairline rgba(237,236,238,0.078)`,
     `cardBorder 0.6 px`.
   - Radii: `radiusCard 12`, `radiusRow 8`, `radiusPill 999`, `radiusTile 4`.
   - Spacing: `padCard = Padding(14,16,14,16)`, gaps 6/8/10/12/14.
   - Type: `fontDisplay = "Segoe UI Variable Display, Segoe UI"`,
     `fontBody = "Segoe UI Variable Text, Segoe UI"`, `fontMono = "Cascadia Mono, Consolas"`,
     `fontPoem = "Georgia, Cambria"`; sizes 10 / 11 / 12.5 / 13.5 / 26 / 32.
   - Helpers `displayText`, `bodyText`, `captionText`, `monoText` that apply family + size +
     color so call sites stop repeating `.FontSize(...).TextColor(...)`.

2. **Wash math** (pure, unexported, testable):
   ```go
   func alphaAt(f float32) float32        // 217→107 over [0,0.5], 107 after
   func composite(bg, wash Color, f float32) Color
   func compositeRGB(bg, wash Color, f float32) Color  // opaque variant
   func sink(c, under Color, k float32) Color
   func lift(c Color, k float32) Color
   func cardGround(bg, wash Color) Color  // composite at 1.0 then sink 0.15
   ```
   Colors as `Color` with 0–255 channels is what `channels_test` can compare; convert with
   `Color.RGBA()` and round independently, as the source does.

3. **Palette rewrite**: each `palette` keeps `wash Color` (Mole's value: clean `#29385c`,
   software `#47201d`, optimize `#423d38`, analyze `#5c3011`, status `#574312`, settings
   `#262c38`) plus the existing orb/bands/tiles/spots/halo. Delete `top`/`bottom` and the
   hand-tuned gradients; `glow` stays for the hero halo.

4. **`View`**: paint `baseBG`, then the wash as a vertical gradient with stops at
   `y=0 → 0.851 alpha` (217/255), `y=0.5 → 0.42`, `y=1 → 0.42` over the palette's wash.
   Keep the radial glow behind content at a lower alpha (≤0.10).
   Cross-fade on tab change with a persistent root element:
   ```go
   root := Box(c).Key("window")
   k := root.AnimateWith("wash", float32(idx), 320*time.Millisecond, EaseInOut)
   pal := mixPalette(paletteFor(allScreens()[prevIdx].ID), paletteFor(allScreens()[idx].ID), k)
   ```
   Store `a.washIdx int` (last laid-out tab index) in `App`; when the index changes, animate
   from the previous one. Headless tests must not animate: when `a.Win == nil`, use the
   direct palette.

5. **Card primitive**: `card` uses `cardGround`, `Radius(12)`, `Padding(14,16,14,16)`,
   `Border(0.6, cardBorder)`. `cardHead(c, ground, icon *SVG, label, meta)` shows a 12×14
   icon tinted by the screen glow, the label at 10 pt bold with `LetterSpacing(0.8)` and
   upper-cased by the caller, then the meta at 11 pt muted. Keep a `cardHeadDot` shim only
   if a call site still passes a dot; otherwise migrate all call sites in this phase.

6. **Row primitive**: `listRow(c, build func())` → `Height(44)`, `Padding(0,6,0,6)`,
   `Radius(8)`, hover `rgba(255,255,255,0.05)`, focused ring; `rowDivider(c)` draws the
   hairline inset by 6 DIPs, drawn only between rows.

7. **Segment primitive**: `segment(c, labels []string, idx *int)` → `Row` pill with
   `Padding(3)`, `Radius(999)`, `Background(rgba(255,255,255,0.06))`, active item white pill
   with dark text and `Shadow(0,1,6,0,rgba(0,0,0,0.25))`, unselected 0.7 white text,
   keyboard tab stops and `Access` labels.

8. **Rail primitive**: replace `bar` internals with a rounded rail: track
   `rgba(255,255,255,0.10)`, fill = linear gradient from `color.Alpha(0.55)` to
   `color.Alpha(0.95)` along the bar, radius `height/2`. Keep the signature.

9. **Nav**: keep the current pill and planet, but adopt tab-bar tokens: fill
   `rgba(255,255,255,0.06)`, stroke `rgba(255,255,255,0.07)`, tab padding 6/14, active pill
   `#f7f5f0`, logo slot 26 with 6 gap.

## Acceptance

- `go test ./internal/ui -run TestDesign` passes vectors:
  - `alphaAt(0)=217/255`, `alphaAt(0.5)=107/255`, `alphaAt(1)=107/255`.
  - `cardGround(#15141b, #29385c)`: derive the expected value by hand once (composite at the
    rest alpha 107/255, then sink 0.15 toward the base — the source's `CardGround` passes
    fraction 1.0, not 217) and pin that constant in the test; a mismatch means a formula
    error, not a colour preference.
  - `sink/lift` clamp at 0 and 255.
- Contrast: for every palette, `fgMuted` on `cardGround` ≥ 4.5:1 and `fgSecondary` ≥ 7:1
  (computed in the test, not eyeballed).
- `.gallery/status.png` shows tinted cards whose hue follows the tab wash; nav unchanged in
  structure.
- Screens build with no new per-frame allocations beyond the existing frame budget.

## Verification

```powershell
go test ./internal/ui -run 'TestDesign|TestShell' -count=1
go vet ./...
go test ./internal/ui -run TestGallery -count=1
```
