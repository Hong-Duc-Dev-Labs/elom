# Verification

The acceptance criteria from the plan, what proves each one, and its state on
this machine (Windows 11, amd64, Go 1.27.3, MyGo v0.8.3, 2026-11-10).

Legend: **verified** = an automated test or a recorded measurement proves it;
**manual** = the code exists and the automated part passes, but a human step
(elevation prompt, Recycle Bin restore, reboot) is still owed; **open** = not
done.

| # | Criterion | Evidence | State |
|---|---|---|---|
| 1 | `go tool mygo build` produces `Elom.exe` and a per-user NSIS installer that installs and uninstalls without administrator rights | `Elom.exe` 11.5 MB, installer 3.9 MB; silent `/S` install and uninstall verified: installs under `%LOCALAPPDATA%\\Programs\\Elom` with no UAC prompt and removes the directory | verified |
| 2 | Clean scans user caches in ≤ 20 s and every item shows path, size and reason; deletions land in the Recycle Bin and restore | 37-rule catalog: 61 candidates, 7.5 GB, 387 ms; per-item guard/whitelist/age reasons tested; trash path unit-tested with a fake trasher | manual (restore) |
| 3 | The guard refuses system paths, junctions and UNC; cleanup refuses to run with an unreadable whitelist or log | `internal/core/safety` table tests (18 cases incl. junction created with `mklink /J`); `svc.Clean.Scan` error tests | verified |
| 4 | Analyze opens `C:\Users\<user>` in ≤ 15 s, drills five levels, lists largest files, trashes a selection | index 261k entries / 40.5 GB in 3.5 s at the default cap (880k / 20.3 s at 1M); drill-down and largest files tested headlessly; layout tests | manual (visual + restore) |
| 5 | Status updates at 1 Hz with 60-sample sparklines, lists processes, uses < 1 % CPU idle; closing the window quits | sampler 18–29 ms per read; idle CPU 0.18 % over 10 s; ring/rate/health tests; no tray, flyout or global shortcut remains, and MyGo quits on the last closed window | manual (sparkline look, close smoke) |
| 6 | Software lists apps within 5 % of Settings and shows leftovers unchecked | 366 registry apps + 131 packaged on this PC; parsing and leftover tests | manual (count comparison + real uninstall) |
| 7 | Startup lists Run keys, StartupApproved and folders; a toggle survives a reboot and reverts | 17 entries with real flags (Discord disabled, Chrome enabled); blob encode/decode tests | manual (reboot) |
| 8 | Optimize shows a dry-run and a skip reason without administrator rights; elevated work is a whitelisted op validated again | unelevated admin op returns `needs administrator rights` (probe); unknown op rejected with exit 1; strict job parsing tests | manual (accept/cancel the UAC prompt) |
| 9 | No crash on unreadable directories or locked files; skips say why | index records per-directory errors; scanner records blocked reasons; delete reports per-item failures | verified (tests) |

## Re-running the checks

```powershell
go vet ./...
go test ./... -count=1
go test ./... -cover -count=1          # 59.5 % total at the time of writing
go tool mygo build                     # build/windows-amd64/
go tool mygo i18n check .              # en + vi catalogs agree
$env:ELOM_GALLERY=1; go test ./internal/ui -run TestGallery -count=1  # visual pass
```

Opt-in probes that read the real machine (skipped by default):

```powershell
$env:ELOM_REAL_SCAN=1;    go test ./internal/svc -run TestRealScan -v
$env:ELOM_REAL_INDEX=1;   go test ./internal/core/scan -run TestRealIndex -v
$env:ELOM_REAL_SAMPLE=1;  go test ./internal/core/metrics -run TestRealSample -v
$env:ELOM_REAL_APPS=1;    go test ./internal/core/apps -run TestRealApps -v
$env:ELOM_REAL_STARTUP=1; go test ./internal/core/startup -run TestRealStartup -v
$env:ELOM_REAL_WINGET=1;  go test ./internal/core/winget -run TestRealWinget -v
```

## Known gaps

- No Windows 10 machine was available: Windows 10 22H2 support is unverified.
- The binary is unsigned; SmartScreen will warn until a certificate is added
  (`windows.certificate` in `mygo.json`).
- The updater is not configured: it needs a signing key and a release feed.
- `main.go` (window wiring) is covered by a launch smoke test, not by unit
  tests.
- The dot globe's 20 Hz frame cost is measured only in the headless software
  rasterizer (about 67 ms for a full 1180x760 clean frame); the GPU backend
  cost in the running app is not measured.
