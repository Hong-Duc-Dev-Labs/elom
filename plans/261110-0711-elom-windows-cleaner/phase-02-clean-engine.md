---
phase: 2
title: "Clean Engine"
status: pending
priority: P1
effort: "2.5d"
dependencies: [1]
---

# Phase 2: Clean Engine

## Goal

Ship the first destructive feature end to end: a declarative rule catalog, a cancellable scanner that sizes every candidate, a review screen, and deletion that goes to the Recycle Bin through the guard and the operation log.

## Context Links

- Guard contract and oplog: [phase-01-spike-skeleton.md](./phase-01-spike-skeleton.md)
- Mole CLI safety wording: https://github.com/tw93/Mole (`--dry-run`, whitelist, oplog)
- MyGo trash semantics: https://mygo.egoist.dev/docs/native.md (The shell)

## Key Insights

- The cleanup catalog must be data, not code: adding an app cache becomes a JSON entry, and the same rule data is reusable by a future CLI.
- A preview is not a dry-run flag here; the review screen *is* the dry run. Nothing deletes without the user seeing paths and sizes.
- Sizes must be measured, never estimated from `EstimatedSize` registry values, or the reclaimable total becomes a lie.
- Recycle Bin has a size limit per drive; `Shell.TrashItem` fails for oversized items, which must surface as a per-item result, not a silent permanent delete.

## Requirements

**Functional**

- Catalog of ~40 rules across categories: `browser`, `app-cache`, `logs`, `dev-cache`, `system-temp`.
- Each rule: id, category, app, paths with environment expansion and `*` globs, `minAgeDays`, `regenerates`, `defaultChecked`, `requiresAdmin`, `note`.
- Scan reports per candidate: path, bytes, file count, last write, checked state, and a skip reason when blocked or unreadable.
- Group by category with subtotal; sort by size; tri-state group checkbox.
- Deletion defaults to Recycle Bin; permanent deletion allowed only for `system-temp` and only when `settings.PermanentDelete` is on.
- Whitelist globs from `settings.Whitelist()`; unreadable whitelist aborts the run before any scan result is shown.
- Every deleted path is one oplog record with result (trashed, permanent, failed + reason).

**Non-functional**

- Scan of a typical profile leaves the UI responsive; progress updates throttled to 10 Hz; cancel stops within 200 ms.
- Worker pool sized `min(runtime.NumCPU(), 8)` per rule directory.
- No symlink traversal; locked files reported, not retried forever.

## Architecture

```
rules/en.json ──► rules.Load() ──► []Rule
                    │
        scan.Run(ctx, rules, whitelist) ──► []Candidate
                    │                       (guard-checked, sized)
                    ▼
        ui.CleanView (review) ──► delete.Trash/Permanent ──► oplog + per-item result
```

Rule JSON shape:

```json
{
  "id": "chrome-cache",
  "category": "browser",
  "app": "Google Chrome",
  "paths": ["%LOCALAPPDATA%\\Google\\Chrome\\User Data\\*\\Cache"],
  "minAgeDays": 0,
  "regenerates": true,
  "defaultChecked": true,
  "note": "Chrome rebuilds the cache; profiles, cookies and history are untouched."
}
```

Glob semantics: `*` matches within one path segment only (Go `filepath.Match` per segment), so a rule can never escape its parent. Rules are expanded against allow roots; any expansion that leaves an allow root is dropped with a logged reason.

## Related Code Files

Create:

- `internal/core/rules/rule.go`, `catalog.go`, `load.go`, `load_test.go`, `data/en.json`
- `internal/core/scan/walk.go`, `size.go`, `candidate.go`, `walk_test.go`
- `internal/core/delete/trash.go`, `permanent.go`, `result.go`, `delete_test.go`
- `internal/svc/clean.go` — scan orchestration, progress events, cancellation, deletion job
- `internal/ui/clean.go` — groups, rows, summary bar, confirm dialog, result toast

Modify:

- `main.go` — mount the Clean view and wire scan/deletion
- `locales/en.json` — Clean strings
- `internal/core/settings/settings.go` — add `PermanentDelete bool`

## Implementation Steps

1. Define `rules.Rule` and the JSON loader with strict decoding (`json.Decoder.DisallowUnknownFields`), validation (non-empty id, category, at least one path, `minAgeDays >= 0`), and duplicate-id detection.
2. Write the v1 catalog (`data/en.json`), embedding it with `//go:embed`. Keep to user-writable locations plus `%TEMP%`; anything requiring administrator rights is deferred to phase 6 and marked `requiresAdmin`.
3. Implement path expansion: environment variables via `os.Expand`, then per-segment glob expansion, then `safety.Guard.Check` on every resulting path; blocked paths become `Candidate{Blocked: reason}` and are visible but not selectable.
4. Implement the scanner: bounded worker pool per rule, `filepath.WalkDir` without following reparse points, `d.Info()` sizes, age filter, cancellation via `ctx`, progress callback invoked at most every 100 ms with counters.
5. Implement `delete.Trash([]Candidate) []Result` calling `mygo.Shell.TrashItem` per path, and `delete.Permanent` guarded to `system-temp`; both refuse when `safety.Guard` disagrees at delete time (re-check, not trust from scan time).
6. Wire `oplog.RequireWritable()` at the start of a deletion run; on failure show a blocking error and delete nothing.
7. Build the Clean view: category groups with subtotal, per-row checkbox and size, "Select all" per group, summary bar ("12.4 GB in 138 items"), confirm dialog naming counts, bytes and the destination (Recycle Bin), and a destructive button style.
8. Add the permanent-delete toggle in Settings, disabled by default, with a confirmation that names the category it applies to.
9. Handle per-item failures: collect `Result` values, show a summary toast ("1 142 of 1 150 items trashed, 8 failed"), and list failures in the oplog viewer.
10. Tests: rule loader (valid, unknown field, duplicate id, glob escaping), scanner on a synthetic temp tree (sizes, age filter, cancel), delete classification with a read-only file and a path outside the allow root.

## Todo

- [x] Rule schema + loader + validation tests
- [x] v1 rule catalog (37 entries) embedded
- [x] Path expansion + guard integration
- [x] Scanner with worker pool, age filter and cancel
- [x] Trash/permanent delete with re-check and results
- [x] Oplog fail-closed wiring
- [x] Clean review UI + confirm dialog + result summary
- [x] Permanent-delete setting (default off)
- [x] Unit tests for loader, scanner, delete classification

## Evidence

Automated (`go test ./...`):

- 37-rule catalog validates; every rule has a note and no hard-coded `C:\` path.
- Scan of the real catalog on this PC: **60 candidates, 7.5 GB, 305 ms** (cached), 3 blocked.
- Guard, whitelist and age filters covered by table tests; deletion re-checks the guard.
- Fail-closed verified both ways: corrupt `whitelist.json` and unwritable `operations.log`
  make `Clean.Scan` return an error, and `delete.Run` leaves every item un-deleted.
- Permanent mode applies only to `system-temp`; app-cache still goes to the trash (test).

Needs a manual pass (not automatable here):

- Deleting a real selection and restoring it from the Recycle Bin (`Shell.TrashItem` needs the
  running app; the unit tests use a fake trasher).
- Cancel mid-scan latency (logic covered; wall time not measured).
- The confirm dialog's look and its destructive-button styling.

## Success Criteria

- [ ] Scan of a real profile finishes in ≤ 20 s and shows per-item path, size and reason.
- [ ] Sizes for three sampled folders match `Get-ChildItem -Recurse | Measure-Object -Sum Length` within 1%.
- [ ] Deleting ~1 GB moves items to the Recycle Bin and every item can be restored in Explorer.
- [ ] Each deletion produces one oplog line whose path and byte count match the item deleted.
- [ ] Making `operations.log` read-only prevents the run and shows an explicit error.
- [ ] A rule pointing at `C:\Windows\Temp` is dropped by the guard, not scanned.
- [ ] Cancel during scan stops within 200 ms and the UI stays responsive.

## Verification

```powershell
go test ./internal/core/rules/... ./internal/core/scan/... ./internal/core/delete/...
go tool mygo dev
# ground truth for one sampled folder:
Get-ChildItem "$env:LOCALAPPDATA\Google\Chrome\User Data\Default\Cache" -Recurse -File |
  Measure-Object -Property Length -Sum
Get-Content "$env:APPDATA\elom\operations.log" -Tail 5
```

## Risk Assessment

| Risk | Mitigation |
|---|---|
| A catalog rule is wrong and deletes user data | Rules restricted to caches/logs; `minAgeDays` for logs; guard allow-list; review screen shows every path |
| Locked files (browser running) fail mid-run | Per-item results; message names the owning app; no retry storm |
| Recycle Bin limit rejects a large folder | Result says "too large for Recycle Bin"; permanent delete offered only as a separate explicit choice |
| Hard links/sparse files inflate sizes | Documented; acceptable for a cache preview; Analyze (phase 3) states the same limitation |
| Scanning `%LOCALAPPDATA%` twice (glob + rule overlap) | Deduplicate candidate paths by cleaned path before sizing |

## Security Considerations

- Deletion re-checks `safety.Guard` at execution time; the review screen is not a trusted input.
- No administrator rights; anything needing elevation is skipped with a reason.
- The oplog records absolute paths — it lives in the user's own profile and is never uploaded.

## Next Steps

Phase 3 reuses the same scanner primitives (`scan/size.go`) for the disk index and the treemap.
