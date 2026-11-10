# Elom

A Windows utility that cleans, uninstalls, monitors and maps a PC, built with
[MyGo](https://mygo.egoist.dev) native UI: Go, no cgo, no webview, one ~10 MB
executable.

Elom is being built in phases; the plan and its acceptance criteria live in
[`plans/261110-0711-elom-windows-cleaner/plan.md`](plans/261110-0711-elom-windows-cleaner/plan.md).
The spike report from phase 1 is in
[`reports/spike-windows.md`](plans/261110-0711-elom-windows-cleaner/reports/spike-windows.md).

## State

| Phase | Scope | State |
|---|---|---|
| 1 | Shell, sidebar, tray, safety guard, operation log, settings, i18n | done |
| 2 | Clean: rule catalog, scanner, review, Recycle Bin deletion | code done, manual pass pending |
| 3 | Analyze: disk index, treemap, largest files | code done, manual pass pending |
| 4 | Status: 1 Hz sampler, sparklines, process table, tray flyout | code done, manual pass pending |
| 5 | Software and Startup: uninstall, leftovers, winget, startup toggles | code done, manual pass pending |
| 6 | Optimize and the elevated job protocol | code done, UAC pass pending |
| 7 | Packaging, Vietnamese, verification | code done, updater not configured |

## Safety model

- **Review before delete.** Cleanup shows paths, sizes and reasons; nothing is
  deleted from a scan result the user has not confirmed.
- **A guard, not a convention.** `internal/core/safety` refuses volume roots,
  `Windows`, program files, personal folders, network paths, alternate data
  streams and anything behind a junction or symbolic link. Deleting code calls
  it again at the moment of deletion.
- **Recycle Bin by default.** Permanent deletion is off unless the user turns
  it on, and applies only to temporary files.
- **Fail closed.** If the protection list or the operation log cannot be read,
  cleanup refuses to run.
- **No elevation in the UI.** Privileged maintenance runs in a second,
  short-lived invocation that re-validates a closed list of operations.

## Build

Requires Go 1.27 or later. Nothing else: the MyGo CLI is pinned as a Go tool.

```powershell
go tool mygo dev      # development build with live reload
go tool mygo build    # build/windows-amd64/Elom.exe + NSIS installer
go test ./...         # core tests and headless view tests
go vet ./...
go tool mygo i18n check .
```

## Layout

```
main.go             app: single instance, window, tray, hide-on-close
internal/core/      pure Go, no MyGo imports (enforced by arch_test.go)
  safety/           the deletion guard
  oplog/            append-only operation log
  settings/         preferences and the protection list
  apps/             installed apps, leftovers and the uninstall runner
  startup/          Run keys, StartupApproved and Startup folders
  winget/           update listing and the tolerant table parser
  maint/            the closed table of maintenance operations and job files
  metrics/          the 1 Hz sampler, history and process list
  rules/            the cleanup catalog (JSON) and its path expansion
  scan/             sizing the catalog and indexing a disk tree
  delete/           Recycle Bin and permanent deletion, with the log
internal/ui/        native UI: sidebar, screens, widgets
locales/            message catalogs (en) and their generated accessors
resources/          app and tray icons
```
