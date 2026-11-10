---
title: Clean and Optimize heroes drop the horizon rule
date: 2026-11-10
summary: Removed the 280 DIP hairline under the hero planet on Clean and Optimize; the globe sits directly above its text and action
---

# Clean and Optimize heroes drop the horizon rule

# What happened

Both hero stages drew a 280 DIP hairline between the dot-matrix globe and the
promise text:

- `internal/ui/clean.go` `cleanHero`: `Box(c).Width(280).Height(1)` under the
  planet, inside a `Gap(10)` column;
- `internal/ui/optimize.go` `optimizePage`: the same rule under its planet.

The rule was removed in both. The planet is now a direct child of the centered
stage, so the 28 DIP stage gap separates the globe from the text without a
decorative line. The comments that described the horizon were updated.

# Evidence

- `go test ./... -count=1` and `go vet ./...` pass.
- `.gallery/clean-empty.png` and `.gallery/optimize.png` regenerated and
  reviewed: globe, two-line promise and CTA remain centered, no divider.

# Decision

Applied to both heroes rather than only Clean because it is one shared visual
element (the horizon under the hero globe); leaving it on Optimize would split
the same stage into two looks. Settings and list hairlines are a different rule
and stay.

# Next steps

- None.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
