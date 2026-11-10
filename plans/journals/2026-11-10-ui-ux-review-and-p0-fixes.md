---
title: ui-ux-review-and-p0-fixes
date: 2026-11-10
summary: "Ten-agent UI/UX review then P0 fixes for clean confirm, uninstall errors, optimize progress, startup errors, and status states"
---

# ui-ux-review-and-p0-fixes

## What happened
- Reviewed the 7 TUI screens with 10 read-only scouts against `internal/ui/design.go` and freshly rendered `.gallery/*.png`; full report delivered in chat.
- Fixed the five P0 groups from that review:
  - Clean: permanent deletion now confirms with its own detail and button (`cleanConfirmFor`, `cleanConfirmPermanentDetail`), instead of promising a Recycle Bin restore.
  - Software: uninstall errors are stored/rendered (`softwareUninstallFailed`), leftovers are skipped after a failed uninstall, and Update/Uninstall stay visible but disabled while busy; apps without an uninstaller explain why.
  - Optimize: the routine marks each task's own op as running, shows Cancel, uses cancellable contexts and `errors.Is`; danger confirm is marked destructive.
  - Startup: toggle failures use `StartupToggleFailed` in a separate `toggleErr`; source labels and the shortcut note are localized; machine-scope rows carry the administrator tag; an elevated write shows a spinner and ignores repeats.
  - Status: the process card keeps an empty-state reason, headers are localized, kill failures are shown, and the health tile/chips use `capsLabel`/`bigNumber`/`pillTag`.
- Added 8 regression tests; added 17 locale keys per language and regenerated `locales/catalog_gen.go`.

## Decision
- Scope stayed at the five recommended P0 fixes; P1 polish (keyboard hints, muted-ramp fork, Settings caps, gallery fixtures for error states) is untouched.

## Verification
- `go tool mygo i18n check .` (182 messages), `go vet ./...`, `go test ./...` pass; `go test -race ./internal/ui -run TestRoutineMarksTheRunningTask` passes.
- Gallery re-rendered and status/startup/optimize/software/clean screenshots inspected.

## Next steps
- Address remaining P1s from the review; add gallery fixtures for error/empty states instead of the `!loaded` shot.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
