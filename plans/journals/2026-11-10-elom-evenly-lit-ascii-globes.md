---
title: "Elom: evenly lit ASCII globes"
date: 2026-11-10
summary: "Removed the terminator from the ASCII planets: texture-only brightness, every cell inside the disc inked, two ink tones — the whole planet is visible."
---

# Elom: evenly lit ASCII globes

## What happened

The ASCII globes lit the body with a fixed upper-left light (`day = smoothstep(-0.12, 0.28, n·L)`)
and only inked cells above ramp index 2. Most of the disc therefore fell into the faint ink and read
as a field of dots behind a lit crescent — the shadow side was hard to see.

- `internal/ui/planet.go`: `globeBrightness` now maps the surface texture alone
  (`b = pow(clamp(0.35 + 0.65·tex), 1.2)`); the light vector, `ndl`, `rim` and `limb` cell fields are
  gone. `drawGlobe` inks every cell inside the disc (`idx = 1 + int(b·(len(ramp)-2))`) and uses the
  palette's mid/lit inks instead of faint/mid/lit, so the body never hits the barely-visible tone.
- `internal/ui/planet_test.go`: geometry test now asserts disc coverage instead of limb cells; new
  `TestGlobeHasNoShadow` fails if any body/angle drops a cell below 0.4 brightness.
- Phase-02 plan, `plan.md` and the renderer comments updated to the new look.

## Verification

`go build ./...`, `go vet ./internal/ui/`, `go test ./internal/ui/ -count=1` pass. Rendered
`.gallery/{clean-empty,clean-review,optimize,analyze,status,startup,software}.png` (1180×760):
Earth, Mercury, Jupiter, the Sun tile and the small globes all show a fully inked disc with no
terminator; only the surface rotates.

## Next steps

None; the rotation, `ReduceMotion` freeze and 20 Hz repaint schedule are unchanged.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
