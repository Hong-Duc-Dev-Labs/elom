---
title: "Elom: review fixes (P1/P2) with measurements"
date: 2026-11-10
summary: "Fixed UI-thread filesystem walks, swallowed winget/enumeration errors, scan-goroutine settings race, oplog per-record open; parity test now sees ICU placeholders."
---

# Elom: review fixes (P1/P2) with measurements

## What happened

Applied the code-review findings to Elom, then verified each repair.

- P1: `internal/ui/software.go` walked the leftover's whole tree inside the view, every frame.
  Sizes are now measured once in `svc.Software.Leftovers` and carried on `apps.Leftover.Bytes`.
- P2: winget updates discarded errors and output; failures now set `updatesErr` and the last
  lines are shown. Registry/packaged enumeration no longer hides failures: `apps.Enumerate`
  errors when no uninstall hive could be read, and `svc.Software.List` returns warnings.
- P2: `analyze.startIndex` read `a.Settings` from the scan goroutine; limits are captured first.
- P2: the operation log opened and synced per record; `oplog.Log.Run` now reuses one handle for
  a run. Measured 0.92 -> 0.42 ms per record (200 records, same fsync durability).
- P2: the catalog parity test's regex missed ICU typed placeholders; it now compares argument
  names, and a new test proves `{count, number, integer}` and `{total, number, integer}` differ.
- P3: quitting before the app is ready no longer calls Stop on a nil app.

## Verification

`go test ./... -count=1` 15/15 packages, `go test -race ./...` clean, `go vet` silent,
`mygo build` 11.5 MB, launch smoke alive. New tests: log writer round-trip and refusal,
software service warnings/failure/measured leftovers, leftover bytes rendered from state,
ICU placeholder comparison.

## Next steps

The remaining manual checks from docs/verification.md are unchanged (UAC accept/cancel,
Recycle Bin restore, startup reboot, winget apply).

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
