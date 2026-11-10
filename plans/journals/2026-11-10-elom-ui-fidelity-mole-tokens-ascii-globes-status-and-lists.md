---
title: "Elom UI fidelity: Mole tokens, ASCII globes, status and lists"
date: 2026-11-10
summary: "Ported Mole's wash/card/type system on MyGo, replaced blob planets with rotating ASCII globes, rebuilt status tiles and list rows; all tests green."
---

# Elom UI fidelity: Mole tokens, ASCII globes, status and lists

## What happened

Implemented `plans/261110-1615-elom-ui-fidelity` (four phases) in `internal/ui`,
plus per-core CPU in `internal/core/metrics`.

- `design.go`: base `#15141b` + tab wash (alpha 217→107), card ground = wash composited and
  sunk 15%, radius 12 / padding 14-16, type scale, mono/display/body font stacks, row and
  divider primitives, gradient rails, per-core bars, dual-series network chart.
- `planet.go` (new): ASCII character globes. Sparse ramp `" ·.:+*"` (Sun `" ·*#%@"`), three
  tones, fixed upper-left light, surface rotating at 20 Hz with `AnimationFrameRate`, limb
  dots, deterministic value-noise bodies, geometry cached by grid size.
- `status.go`: 4×2 tiles of 164 px, per-core CPU, dual network fills, rails, glyph + memory +
  CPU rail + PID + action table.
- `apps.go`/`software.go`: white segmented pill, 44 px rows with dividers, inline expandable
  detail, monospace size.
- `analyze.go`: treemap labels top-left (icon, name, mono size), gradient fill, hover ring;
  sidebar rows on the shared primitive.
- `shell.go`: settings rebuilt as one card of label/description rows with section caps.
- `metrics`: `Raw.Cores`/`Snapshot.Cores`, `History.NetDown/NetUp`, one per-core gopsutil call
  per sample (mean is the aggregate).

## Decisions

- No Mole asset, glyph path, texture or copy is used; the stash forbids it and Elom is MIT.
- Wash cross-fade is a manual 320 ms blend; `AnimateWith` starts new keys at their target.
- Process action is the row button (existing confirmation), no popover, keeping `ui.Table`.
- Battery rings, GPU and thermal tiles stay out (no data source).

## Evidence

`go test ./...` green; `go vet ./...` and `go tool mygo vet .` silent; `i18n check` 168
messages; `BenchmarkGlobeShading` 0.31 ms/op, 0 allocs; gallery regenerated and reviewed;
app launches and stays alive for a 5 s smoke.

## Next steps

Run a live pass with `go tool mygo dev` to tune globe density per body and confirm the wash
cross-fade on tab switches.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
