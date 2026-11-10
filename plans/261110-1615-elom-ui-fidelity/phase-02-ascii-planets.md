---
phase: 2
title: "ASCII planets with 3D rotation"
status: done
effort: "1.5d"
depends: [1]
---

# Phase 2 — ASCII planets with 3D rotation

Goal: replace the painter's gradient sphere + coloured blobs (`design.go` `planet`,
`appMark`) with a **character-rendered globe** in the style of the Devault ASCII generator's
"Classic" preset: one glyph per cell chosen by brightness, a fixed light, and the surface
rotating in 3D underneath.

Decision (2026-11-10, user): do not reproduce Mole's photographic/D3D planets. Own the
start: ASCII art is unique, cheap, asset-free, and animates with the painter.

Reference: `PlanetStyle.cs` only for which body belongs to which tab and for rotation
periods; the look comes from `https://devault-ascii.pages.dev/ascii-generator/` (Classic:
brightness ramp, edges, colour, glow).

## Look

Concept sheet (own renderer, validated): `assets/ascii-planets-concept.png` — Earth, Mars,
Mercury, Jupiter, Sun on the top row; three Earth rotation angles on the bottom row.

- Grid of monospace cells painted with `Painter.RichText` per row, two tone passes.
- Sparse ramp for the bodies: `" ·.:+*"` (6 levels). Sparse is what keeps a 64-column globe
  legible — dense ramps (`#`, `@`) turn the disc into a grey block.
- Sun only: denser emissive ramp `" ·*#%@"`.
- The body's texture decides *ink density*: land/bands/ice brighter and denser, ocean faint.
  There is no light terminator: `b = 0.35 + 0.65·tex`, so the whole disc keeps enough ink
  to read at a glance (revision 2026-11-10: the shadow side was hard to see).
- Every cell inside the disc is inked — no blank cells and no night side. Only the surface
  rotates; the visible disc stays fixed.
- Two ink tones per body (base / lit) picked from `b` (0.78 threshold), using the palette's
  mid and lit entries. Ink: Earth `#8fbdb0`/`#e9f7f0`, Mars `#b4694f`/`#f2c2a8`, Mercury
  `#8f8f88`/`#e3e3da`, Jupiter `#c08e5a`/`#f6dfb8`, Sun `#e0a13a`/`#fff0b8`, Moon
  `#8f8f86`/`#efece1`.
- Per-body texture (own math, no images): Earth fBm continents + ice caps, Mars crater
  fields, Mercury grey craters, Jupiter latitude bands + storm oval, Sun emissive
  granulation (no terminator), Moon neutral.
- Behind the globe: the palette glow at ≤ 0.18 alpha; nothing else.

## Rendering

```go
// planet draws the globe: a grid of cells, each a glyph of the ramp.
// Light and cell geometry are fixed; only the texture longitude turns.
func planet(c *Context, size float32, pal palette) Element {
	return Box(c).Size(size, size).Draw(func(p *Painter, r Rect) {
		drawGlobe(p, r, pal, globeAngle(p, pal))
	})
}
```

Per frame, per cell (`u,v` in `[-1,1]`, disc test `u²+v² ≤ 1`):

1. `z = sqrt(1-u²-v²)`; tilt the point about X (`≈20°`), then rotate about Y by `angle`.
2. `lat = asin(y)`, `lon = atan2(x, z')`; sample the body texture at `(lon, lat)`.
3. `b = pow(clamp(0.35 + 0.65·tex, 0, 1), 1.2)` — texture only, no light terminator.
4. `idx = 1 + int(b·(len(ramp)-2))`, clamped to `len(ramp)-1`: every cell inside the disc
   is inked, so the ramp's blank and faintest levels are never picked.
5. Two row strings per row — one per tone — so `RichText` draws whole runs and positions
   never drift: each pass holds its own cells and spaces elsewhere.

Cell metrics: `cols = clamp(round(size/5.0), 16, 72)`; `cellW = size/cols`;
`fontSize = cellW·1.6`; `cellH = cellW·1.68`; `rows = floor(size/cellH)`; centre the grid.
Hero globes grow to 320 px (from 232) so a 64-column globe stays legible; smaller
placeholders use 160 px (32 columns). Measure the mono advance once (`MeasureText` on
`"MMMMMMMMMM"`) and adjust `cellW` so rows never drift or wrap.

Static cache: the cell frame (`u,v,z,inside`) depends only on
`(cols, rows, tilt)` — cache it in a small `sync.Map`/package map. Per frame only the
rotated longitude lookup and ramp index are computed.

## Animation

- `angle = p.Now()` seconds × speed; Earth 0.10 rad/s, Mars 0.14, Mercury 0.06,
  Jupiter 0.18, Sun 0.05.
- Inside `Draw`: `if animate { p.AnimationFrameRate(20) }`; no view rebuild, the painter
  repaints the element only (confirmed in `paint.go`: `AnimationFrameRate` repaints without
  building the view).
- Static frame when `p.Preferences().ReduceMotion` is true.
- **Only hero globes animate** (Clean, Optimize, Analyze, Apps/Startup empty states).
  Status health tile and the nav mark render a fixed angle: the Status screen keeps its
  1 Hz sampler and its idle-CPU promise.
- Hero extras: a soft radial glow behind the globe (`pal.glow`, ≤ 0.18 alpha) and a
  1 px faint baseline for the "ground" of the ASCII field.

## Files

| File | Change |
|---|---|
| `internal/ui/planet.go` | **new**: ramp, bodies, noise, globe geometry, cache, `drawGlobe`, `globeAngle` |
| `internal/ui/planet_test.go` | **new**: geometry/cache/tone/angle/determinism, benchmark |
| `internal/ui/design.go` | `planet` becomes the ASCII element; `appMark` stays a small solid disc; delete blob code and `orbSpot`/`bands`/`clouds` |
| `internal/ui/clean.go`, `optimize.go`, `analyze.go`, `apps.go`, `startup.go` | no call-site change (same `planet(c, size, pal)` signature) |
| `internal/ui/status.go` | health tile uses `planet(..., static=true)` or a fixed angle |

## Acceptance

- `.gallery/clean-empty.png`, `optimize.png`, `analyze.png` show the sparse character globe
  of `assets/ascii-planets-concept.png`, not a smooth sphere or a grey block; the silhouette
  is a clean circle of `·` at the limb and continents read as bright clusters.
- Rotating: in `go tool mygo dev`, the surface visibly turns while the lit side stays put;
  `ReduceMotion` freezes it; leaving the screen (or hiding the window) stops repaints.
- Status does not animate (fixed angle) and its idle CPU stays under the existing budget.
- `planet_test.go` asserts: no glyph outside the disc; at least three tone buckets present
  on a lit Earth; `globeAngle` advances linearly with `Now`; the geometry cache returns the
  same slice for the same `(cols, rows)`; two renders at the same angle are identical
  (deterministic noise).
- `BenchmarkGlobe` paints a 52×30 grid in ≤ 3 ms on the dev machine (20 fps budget is
  50 ms/frame; the globe must be a small slice of it).
- No asset file added; no new dependency.

## Verification

```powershell
go test ./internal/ui -run 'TestPlanet|TestGlobe' -count=1 -v
go test ./internal/ui -bench BenchmarkGlobe -benchtime 20x -run '^$'
go test ./internal/ui -run TestGallery -count=1
go tool mygo dev   # look at the rotation, check Task Manager CPU on Status
```
