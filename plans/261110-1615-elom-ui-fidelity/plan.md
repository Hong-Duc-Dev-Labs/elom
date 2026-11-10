---
title: "Elom UI fidelity pass — Mole's design system on MyGo"
description: "Port Mole for Windows' measured UI system (tab wash, card metrics, type scale, ASCII character globes with 3D rotation, status grid, row/segment primitives, treemap labels) into Elom's MyGo UI without copying any third-party asset or text."
status: pending
priority: P2
effort: "~7d"
tags: [ui, design-system, mygo, go, windows, mole]
blockedBy: []
blocks: []
created: 2026-11-10
---

# Elom UI fidelity pass

## Overview

[`plans/reports/261110-mole-ui-port.md`](../reports/261110-mole-ui-port.md) compares Elom's
MyGo UI with the decompiled Mole 0.21.2 UI and extracts its measured tokens. Elom already
speaks Mole's language (dark per-screen gradient, floating tab pill, translucent cards,
hero planet); the gaps are the *system*: wash compositing, card metrics, type scale, planet
rendering, status chart shapes, list rows, treemap cells and settings rows.

Outcome: every screen uses one token set and four shared primitives (card, row, segment,
rail), planets are rotating character globes instead of gradient blobs, and the status/lists/treemap
read like the reference shots. No third-party asset, glyph path, texture, copy or poem is
copied — the source stash forbids it and Elom is MIT.

## Constraints

- MyGo 0.8.3 native painter only: no acrylic, no D3D, no WebView2.
- No new dependency. `image`, `math`, `math/rand` from the standard library only.
- `internal/core` stays free of MyGo imports (`arch_test.go`).
- No behaviour change to scanning, deletion, elevation, or safety.
- Existing tests keep passing; view tests keep using `ui.NewTester`.

## Decisions

| # | Decision | Source's way | Our way | Risk | Choice |
|---|---|---|---|---|---|
| 1 | Visual assets | 106 SVGs, planet JPG/PNG, logo | keep our 24-grid line icons; draw planets procedurally | high (licence) | own assets only |
| 2 | Background + cards | base + tab wash (alpha 217→107), acrylic cards | base + wash ramp in one painter gradient; card fill = wash sunk 15% | low | port the math |
| 3 | Planets | D3D textured spheres | ASCII character globe: brightness ramp, 3D surface rotation, fixed terminator, limb dots | low | own ASCII globe |
| 4 | Cards | radius 12, 16/14 padding, 0.6 hairline, 10 pt tracked caps, 26/13 value | same numbers in `design.go` | low | adopt |
| 5 | Status grid | 4×2 fixed 164 px + process table | 4 cols, 164 px rows, 8 tiles, custom process rows | low | adopt |
| 6 | Settings | modal overlay + segmented tabs | one card, label/description rows, control right; overlay primitive only if phase 4 needs it | low | rows, no overlay |
| 7 | Per-process icons | shell icon per PID | neutral glyph tile; no per-second `Exe()` calls | low | glyph |
| 8 | Per-core CPU | per-core strip | extend `metrics` with `Cores []float64` (one gopsutil call, no extra latency) | low | extend sampler |
| 9 | Wash animation | cross-fade on tab change | `Element.AnimateWith("wash", idx, 320ms, EaseInOut)` + `mixPalette` | low | animate |
| 10 | Fonts | Segoe UI Variable / Cascadia / Georgia | same stacks with Win10 fallbacks | low | adopt |

## Dependency matrix

| Source | Elom target | Status |
|---|---|---|
| `Surface.cs` / `TabWash.cs` / `TabWashes.cs` | `internal/ui/design.go` | CONFLICT → phase 1 |
| `HudTile.cs` | `internal/ui/status.go` | EXISTS → phase 3 |
| `HealthCard.cs` | `status.go` `healthTile` | EXISTS → phase 3 |
| `Sparkline/NetworkSparkline/PerCoreStrip/CapacityRail` | `design.go` charts | EXISTS → phases 1/3 |
| `PlanetStyle/Renderer` | `design.go` `planet`/`appMark` | CONFLICT → phase 2 |
| `ProcessTable/ProcessRowView` | `status.go` `processTable` | CONFLICT → phase 3 |
| `AppRowView` | `apps.go`, `software.go` | EXISTS → phase 4 |
| `TreemapCanvas` | `analyze.go` `paintTreemap`, `treemap.go` | EXISTS → phase 4 |
| `CleanPage/CleanReviewOverlay` | `clean.go` | EXISTS → phase 4 (rows only) |
| `SettingsOverlay` | `shell.go` `settingsPage` | CONFLICT → phase 4 |
| `MenuCard/DetailsOverlay` | — | NEW, optional → phase 4 |

## Phases

| # | Phase | Scope | Est. | Status |
|---|---|---|---|---|
| 1 | [Tokens, shell, primitives](./phase-01-tokens-and-shell.md) | wash, card/row/segment/rail, type scale, fonts, nav cross-fade | 1d | Done |
| 2 | [ASCII planets](./phase-02-ascii-planets.md) | character globe renderer, 3D rotation, animation policy | 1.5d | Done |
| 3 | [Status grid and processes](./phase-03-status-grid-and-processes.md) | 164 px tiles, per-core bars, dual network, rails, process rows | 2d | Done |
| 4 | [Lists and surfaces](./phase-04-lists-and-surfaces.md) | apps rows/segments/search, treemap labels + cell fill, settings rows, overlays if needed | 2.5d | Done |

## Acceptance

1. `View` paints base `#15141b` with the active tab's wash at alpha 217 → 107; every card's
   fill is that wash sunk 15% toward the base; switching tabs cross-fades ≤ 320 ms.
2. `design_test.go` pins `composite`, `sink`, `lift`, `alphaAt` against vectors copied from
   `TabWash.cs` (values, not code).
3. Planets are character globes drawn in `Painter.Draw`: geometry cached per grid size, surface
   rotating at 20 fps, terminator and limb fixed; `ReduceMotion` and the Status screen render a
   static frame (`planet_test`, `BenchmarkGlobe`).
4. Status: 4×2 grid of 164 px tiles; CPU shows per-core bars; network shows down/up fills;
   memory and free space are rounded rails; process rows show name, mono memory, CPU rail +
   percent, mono PID, and a row menu.
5. Lists: Apps/Startup/Analyze rows share one primitive (icon, name, mono size, chevron);
   apps rows expand to their uninstall facts; treemap cells keep their 3 px gaps, gain a
   top-left icon/name/mono-size label and a light-to-dark fill;
   Settings is one card of label/description rows.
6. `go test ./...`, `go vet ./...`, `go tool mygo i18n check .` pass; `.gallery/*.png`
   regenerated and visually reviewed against `D:/Dev/reverse/mole/ui/shots/`.
7. No file from `D:/Dev/reverse/mole/ui/assets` is copied into the repo (grep for the
   asset names in `resources/`, `internal/ui/`).

## Verification

```powershell
go test ./internal/ui ./internal/core/metrics
go vet ./...
go tool mygo i18n check .
go test ./internal/ui -run TestGallery -count=1   # writes .gallery/*.png for the visual pass
go tool mygo dev                                  # live: tab wash cross-fade, 1 Hz status
```

## Risks

| Risk | Mitigation |
|---|---|
| Licence violation by borrowing assets | decisions 1 and acceptance 7; own drawing only |
| Procedural planet reads worse than the current blobs | phase 2 gate: screenshot review before deleting the old painter path |
| Wash compositing makes text contrast fall below 4.5:1 | compute card ground per tab and assert contrast in a unit test |
| Per-core sampling doubles CPU cost | one `cpu.PercentWithContext(ctx, 0, true)` per second, measured in `SampleCost` |
| Custom process rows lose table accessibility | keep `ui.Table` semantics where possible; else `Access` labels + keyboard |
| Regressions in headless view tests | update expectations only where behaviour intentionally changed |

## Non-goals

Licence/onboarding/Doctor/What's New/About, poems, Clean Screen media, acrylic blur, D3D,
third-party glyph paths, per-process shell icons, tray flyout redesign.

## Delivered (2026-11-10)

All four phases are implemented in `internal/ui` and `internal/core/metrics`.

| Evidence | Result |
|---|---|
| `go test ./...` | green (ui, metrics, core, svc, locales) |
| `go vet ./...`, `go tool mygo vet .` | silent |
| `go tool mygo i18n check .` | 168 messages, en/vi |
| `go test ./internal/ui -bench BenchmarkGlobeShading` | 0.31 ms/op, 0 B/op, 0 allocs/op for a 64×32 globe |
| `.gallery/*.png` | regenerated and reviewed: clean, clean-review, software, startup, analyze, status, optimize, settings |

Deviations from the draft plan, all deliberate:

- The wash cross-fade is a 320 ms manual blend driven by `Context.AnimationFrame`, not
  `AnimateWith` (a new animation key starts at its target, so it cannot fade in).
- The process row menu is the row's action button (opens the existing confirmation), not a
  popover; the table keeps `ui.Table` for keyboard and screen-reader behaviour.
- Battery device rings, GPU and thermal tiles are not ported (no data source); battery keeps
  its percent and rail.
- The app detail expands inside the list row stack (no overlay primitive); the treemap keeps
  its 3 px gaps and gains the top-left label and gradient fill.
