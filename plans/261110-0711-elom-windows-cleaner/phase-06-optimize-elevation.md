---
phase: 6
title: "Optimize & Elevation"
status: pending
priority: P2
effort: "1.5d"
dependencies: [1, 5]
---

# Phase 6: Optimize & Elevation

## Goal

Add safe maintenance tasks and the only privilege boundary in the app: an elevated invocation of the same executable that validates a closed operation list, performs the privileged work, and reports back — while the UI keeps running unelevated.

## Context Links

- Oplog and guard: [phase-01-spike-skeleton.md](./phase-01-spike-skeleton.md)
- Startup HKLM writes routed here: [phase-05-apps-startup.md](./phase-05-apps-startup.md)
- Mole's maintenance scope and skip reasons: https://mole.fit/windows/blog/what-windows-optimization-does
- MyGo single-instance and lifecycle: https://mygo.egoist.dev/docs/app.md

## Key Insights

- Relaunching the whole UI elevated would lose drag-and-drop from Explorer into the window (UIPI) and put every file operation in a privileged process. A second, short-lived invocation with a job file is smaller and auditable.
- The elevated process must re-validate: the job file is written by an unelevated process, so it is untrusted input until it passes the same guard and a closed op table.
- A maintenance op that needs admin must say so *before* it runs, not fail halfway; "skipped: needs administrator" is a feature.
- `ShellExecuteW` with the `runas` verb is the only UAC path needed; the same binary decides its mode from `--elevated-job`.

## Requirements

**Functional**

- Job file: JSON v1 at `%TEMP%\elom-job-<uuid>.json` = `{v, createdAt, ops:[{op, args…}]}`; result written to `%TEMP%\elom-job-<uuid>.result.json` and appended to the oplog by the elevated side.
- Closed op table: `dns.flush`, `cache.icon`, `cache.thumbnails`, `cache.delivery-optimization`, `temp.system-files`, `update.cache`, `startup.set-hklm`, `disk.optimize`. Unknown ops are rejected with a non-zero exit.
- UI: task list with a dry-run description, admin badge, current state, and per-op result; one UAC prompt per session (remembered until app exit).
- Maintenance ops available: DNS flush (runs unelevated, `ipconfig /flushdns` equivalent), icon cache, thumbnail cache, Delivery Optimization cache, system temp files older than 7 days, Windows Update download cache (advanced, off by default), `defrag`/`Optimize-Volume` on the system drive, SFC/DISM read-only report.
- HKLM startup toggle from phase 5 goes through `startup.set-hklm`.

**Non-functional**

- Elevated invocation lifetime under 60 s per op set; result file polled with a 120 s timeout.
- No elevation at startup; prompt only when the user starts an admin task.
- The elevated process shows the same cancellable progress window and exits when done.

## Architecture

```
UI (unelevated)                        elom.exe --elevated-job <path>
  │  build job (validated ops)             │
  ├─ write job.json to %TEMP%              ├─ read job.json (strict decode, size cap 64 KiB)
  ├─ ShellExecuteW("runas", exe, args) ────┤ validate: known op, args, guard.Check
  ├─ poll result.json                      ├─ run op, collect Result{Op, OK, Err, Bytes}
  │                                        ├─ append oplog records
  └─ show results                          └─ write result.json, exit
```

Threat model: the job file can be replaced by any process of the same user before launch, so every op is treated as untrusted input. Paths in `temp.system-files` are re-expanded from the op's parameters and guard-checked in the elevated process; `startup.set-hklm` accepts only `{hive, name, enabled}`, not a path.

## Related Code Files

Create:

- `internal/win32/elevate_windows.go` — job write, `ShellExecuteW` runas, result polling
- `internal/win32/job.go`, `job_test.go` — schema, strict decode, op validation table
- `internal/core/maint/tasks.go`, `dns.go`, `caches.go`, `tasks_test.go` — op implementations and dry-run text
- `internal/svc/optimize.go` — task orchestration, admin session state
- `internal/ui/optimize.go` — task list, badges, progress, results
- `main.go` — `--elevated-job` mode branch before UI startup

Modify:

- `internal/core/startup/approved.go` — route HKLM through the job
- `locales/en.json` — Optimize strings

## Implementation Steps

1. Define the job schema and the op registry as a table of `{Name, Args, Validate, DryRun, Run, NeedsAdmin}`; validation is pure and unit-tested (unknown op, wrong arg count, path inside a protected root, oversized job).
2. Implement job file write with `os.CreateTemp` in `%TEMP%`, mode 0600 where Windows permits, and a UUID name; delete after result is read.
3. Implement `elevate.Run(ctx, ops) (Result, error)`: write job, `ShellExecuteW(nil, "runas", exe, "--elevated-job <path>", nil, SW_HIDE)` via `golang.org/x/sys/windows` `LazyDLL("shell32.dll")`, detect `ERROR_CANCELLED` (user refused UAC) and return a distinct error the UI shows as "cancelled", then poll for the result file.
4. Implement `main.go` branch: `--elevated-job` parses the job, runs ops, writes the result, exits — never opens a window, never starts the sampler.
5. Implement ops: `dns.flush` (`DnsFlushResolverCache` from `dnsapi.dll`, verifying with `ipconfig /displaydns` in the test), `cache.icon` and `cache.thumbnails` (delete `%LOCALAPPDATA%\Microsoft\Windows\Explorer\iconcache_*.db` and `thumbcache_*.db`, then restart Explorer only after an explicit confirm), `cache.delivery-optimization` (`Delete-DeliveryOptimizationCache` via the job's PowerShell call), `temp.system-files` (files older than 7 days under `%SystemRoot%\Temp`, guard-checked), `update.cache` (stop `wuauserv`/`bits`, clear `SoftwareDistribution\Download`, restart — marked advanced), `disk.optimize` (`defrag C: /O`), `update.report` (`sfc /verifyonly`, `DISM /Online /Cleanup-Image /CheckHealth` read-only).
6. Dry-run text per op: what it touches, how much space it may free (measured when possible), what it costs (e.g. "Explorer restarts", "Windows Update re-downloads").
7. Optimize UI: grouped task list, admin badge, dry-run expander, run button per task, session-level "Administrator access granted" state, results with per-op outcome and skip reasons.
8. Wire `startup.set-hklm` from phase 5 through the same path, keeping the phase 5 revert semantics.
9. Tests: job validation table; op registry completeness (every op has dry-run text and an admin flag); result round-trip; UAC-cancel handling with an injected launcher.

## Todo

- [x] Job schema + strict decode + validation tests
- [x] `ShellExecuteW` runas launcher + result polling + cancel handling
- [x] `--elevated-job` mode in `main.go`
- [x] Op registry with dry-run text and admin flags
- [x] Ops: DNS, icon/thumbnail cache, Delivery Optimization, system temp, update cache, disk optimize, SFC report
- [x] Optimize UI with admin badges and outcomes
- [x] Startup HKLM toggle routed through the job
- [x] Tests for validation, registry completeness, unelevated refusal, bad job file

## Evidence

Probes against the built `Elom.exe`:

| Probe | Observed |
|---|---|
| `--elevated-job bad.json` (unknown op) | exit **1**, stderr `maint: unknown operation: "format.c"`, no result file, no window |
| `--elevated-job good.json` (dns.flush, unelevated) | result `{"op":"dns.flush","ok":false,"error":"maint: needs administrator rights"}` |
| `ipconfig /flushdns` run directly | exit 1, **"The requested operation requires elevation."** |

The last line changed the plan: `dns.flush` was marked unelevated, but Windows 11 here
refuses it without administrator rights, so the op table now marks it `Admin: true`
and `maint.Run` refuses every admin op unless `Token.IsElevated()` is true — a job
file cannot make the app do privileged work by asking nicely.

Automated (`go test ./...`, 14 packages): op table has no duplicates and every task
has a timeout; validation rejects unknown ops, stray args and a bad
`startup.set-hklm` payload; `ParseJob` rejects unknown fields, version ≠ 1, an empty
list, unknown ops and files over 64 KiB; every admin op refuses unelevated.

Machine-wide startup entries now go through the elevated job: the UI builds
`startup.set-hklm` (`startupOp`) and calls the injected `Elevate` (default `svc.RunElevated`);
the payload is validated by the job parser before it is sent, and a test proves the entry
never falls back to a direct registry write.

Needs a manual pass:

- Accept a UAC prompt and watch a task complete (the launcher and result polling are
  implemented but cannot be exercised without the prompt).
- Cancel the UAC prompt and confirm "cancelled" is reported without an error dialog.
- Route the machine-wide startup toggle from the Startup screen through the job.

## Success Criteria

- [ ] Without elevation every admin task shows "needs administrator" and changes nothing.
- [ ] With elevation one UAC prompt appears, tasks run, and results are shown per op.
- [ ] `ipconfig /displaydns` is empty right after a DNS flush.
- [ ] An unknown op in a hand-written job file makes the elevated process exit non-zero with a named error and no filesystem change.
- [ ] A job pointing `temp.system-files` at `C:\Windows\System32` is refused by the elevated guard.
- [ ] Refusing the UAC prompt returns "cancelled" without an error dialog.
- [ ] Every elevated op appends oplog records that the UI reads back and displays.

## Verification

```powershell
go test ./internal/win32/... ./internal/core/maint/...
go tool mygo dev
ipconfig /displaydns | Select-String 'Record Name' | Measure-Object
# refusal path:
& build\windows-amd64\elom.exe --elevated-job "$env:TEMP\bad-job.json"; $LASTEXITCODE
```

## Risk Assessment

| Risk | Mitigation |
|---|---|
| A tampered job file performs privileged deletion | Closed op table, argument schemas, guard re-check in the elevated process, size cap; no arbitrary path lists except guarded temp/cache roots |
| Explorer restart annoys the user | Explicit confirm naming the restart; icon/thumbnail tasks are opt-in, not part of a default sweep |
| Windows Update cache clearing breaks an in-flight update | Off by default, labelled advanced, stops and restarts services in a `defer` even on error |
| NSIS per-user install means the elevated process is the same user | Accepted: elevation is per-task, and the helper writes to the same oplog the UI reads |
| `ShellExecuteW` returns before the child finishes | Poll for the result file with a timeout and surface "still running" instead of a false success |

## Security Considerations

- The job file is untrusted: strict decode, size cap, closed ops, per-op schema validation, guard re-check.
- The elevated process never loads settings or the whitelist from disk; it uses only job parameters and fixed roots.
- No shell string interpolation of user input into PowerShell; arguments are passed as a JSON payload to a fixed script.
- UAC cancellation is a normal outcome, not an error dialog.

## Next Steps

Phase 7 packages the app, adds Vietnamese, Clean Screen, signing/updater configuration and the final acceptance sweep.
