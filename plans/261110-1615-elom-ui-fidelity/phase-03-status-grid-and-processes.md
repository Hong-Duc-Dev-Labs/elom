---
phase: 3
title: "Status grid and process rows"
status: done
effort: "2d"
depends: [1, 2]
---

# Phase 3 — Status grid and process rows

Goal: the Status screen reads like the reference shot: 4×2 tiles of 164 px, icon + tracked
caps header with right meta, one 26 pt reading, one chart built for its metric, one footer
line; then a process table with memory in mono, a CPU rail, a mono PID and a row menu.

Reference: `StatusPage.cs` (8 tiles, rows 164/164/star, tints from resource colours),
`HudTile.cs` (card metrics), `ProcessTable.cs` / `ProcessRowView.cs` (row layout),
`Sparkline/NetworkSparkline/PerCoreStrip/CapacityRail` (chart shapes), shot `08-status.png`.

## Data change

Per-core CPU exists in gopsutil and costs one call per sample (cumulative since the last
call), so the sampler gains it without extra latency:

```go
// metrics/sampler.go
type Raw struct { …; Cores []float64 }          // per logical processor, percent
type Snapshot struct { …; Cores []float64 }     // same, published
type History struct { CPU, Mem, Disk, Net []float64 }
```

`GopsutilReader.Read` calls `cpu.PercentWithContext(ctx, 0, true)` once and fills `Raw.Cores`
(the aggregate CPU may stay as is). The sampler copies it; `sampler_test.go`'s fake reader
gets a `Cores` field and asserts the published snapshot.

## Files

| File | Change |
|---|---|
| `internal/core/metrics/sampler.go` | `Cores` on `Raw` and `Snapshot` |
| `internal/core/metrics/gopsutil_reader.go` | one per-core call per sample |
| `internal/core/metrics/sampler_test.go` | fake reader + assertion |
| `internal/ui/status.go` | tiles, charts, process table |
| `internal/ui/design.go` | `perCoreBars`, `netChart`, rail gradient (if not in phase 1) |
| `internal/ui/status_test.go` | update expectations for the new tree |

## Tasks

1. **Tile**: `Row`/`Column` card of fixed `Height(164)`, `cardGround`, radius 12, padding
   14/16, gap 6:
   - header: 12×14 icon tinted per metric, 10 pt bold upper-cased label, spacer, right meta
     at 11 pt muted (cores, pressure chip, capacity, Wi-Fi none);
   - reading: `bigNumber` at 26 bold display font + 13 semibold unit (memory `%`, CPU `%`,
     free space value+unit, network rate, disk rate, battery `%`);
   - chart slot fills the rest;
   - footer 11 pt muted, one line, ellipsised.
2. **Metric tints** (own values, same idea as the source's planet inks): CPU mint `#4fd6a9`,
   memory amber `#e3c27a`, free space `#7aa7ff`, network `#6fa0d8` + up `#8fd0c4`,
   disk `#8fa8ff`, battery `#8ad9a6`, keep-awake `#f2c078`, health planet sun.
3. **Charts**:
   - `perCoreBars(c, cores, 34)`: one bar per logical processor, min height 15%, gradient
     `alpha 0.55 → 1.0`, width `min(10, slot-3)`, radius 2, faint baseline row.
   - `netChart(c, down, up, histDown, histUp, 34)`: two monotone areas, down mint above/under
     fill 0.28→0.02, up blue line 1.5; shared max; flat baseline at 1 px when history has
     < 2 samples.
   - memory `rail(fraction, color, 14)`; free space `rail(usedFraction, color, 8)`.
   - disk activity `lineChart` stays, footer read/write.
   - battery: percent + `rail(percent, success, 6)` + plug state (rings are **not** ported).
4. **Grid**: `Grid(c).Columns(4).Gap(12)` with fixed 164 px children; health tile keeps the
   96 px planet bitmap (phase 2) on the right, score left with the health word in
   `Success`/`Warning`/`Danger`.
5. **Process table**: keep `ui.Table` (virtualised, accessible) and replace cell content:
   - col NAME: 16 px neutral glyph tile + name at 13 medium, single line;
   - col MEM: mono 12 right-aligned, e.g. `2.6 GB`;
   - col CPU: rail 3 px (width 44) + value at 12 mono, right;
   - col PID: mono 12 muted, right;
   - col menu: `iconButton(iconMore)` opening a `Popover` with `statusEndTask`.
   Column widths: NAME flexible, MEM 90, CPU 110, PID 80, menu 28. Header titles keep the
   existing locale strings, upper-cased at build time.
6. Keep the tray panel (`PanelView`) on the phase-1 primitives; do not port new content.

## Acceptance

- Status renders exactly 8 tiles in 4 columns of 164 px, no tile shorter than its content
  (a 1216×780 window shows two full rows and the process header).
- CPU tile shows as many bars as `snap.Cores` (12 cores → 12 bars), each 0–100%.
- Network tile shows two series and does not draw a filled rectangle when one sample exists.
- Process rows show glyph, name, memory, CPU rail + %, PID, menu; "End task" from the menu
  calls the same `endTask` path and the confirmation dialog still appears.
- Sampler publishes per-core values; `SampleCost` does not regress by more than 1 ms
  (measure before/after with the existing test or a one-off benchmark).
- `status_test.go` asserts the tile count, the per-core bar count and the process cell text
  by rendering headless — not by walking internal state.

## Verification

```powershell
go test ./internal/core/metrics -count=1
go test ./internal/ui -run 'TestStatus|TestGallery' -count=1
go vet ./...
go tool mygo dev   # 1 Hz live pass: CPU bars move, network fills, process menu works
```
