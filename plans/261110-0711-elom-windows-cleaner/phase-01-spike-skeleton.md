---
phase: 1
title: "Spike & Skeleton"
status: completed
priority: P1
effort: "2d"
dependencies: []
---

# Phase 1: Spike & Skeleton

## Goal

Prove that MyGo native UI covers every screen Elom needs on Windows, then stand up the shell, the safety core, the operation log, settings, and i18n that every later phase builds on.

## Context Links

- Product contract and verified framework APIs: [plan.md](./plan.md)
- Upstream: https://mygo.egoist.dev/docs/ui.md, https://mygo.egoist.dev/docs/menus.md, https://mygo.egoist.dev/docs/native.md
- Mole safety model reference: https://github.com/tw93/Mole (README → Safety)

## Key Insights

- MyGo native UI has no webview: one binary (~7 MB vendor figure), Direct3D 11 with WARP fallback, no WebView2 dependency. That is the whole reason the native route was chosen, so the spike must confirm the widgets before any feature work.
- `ui.Table` and `ui.Outline` build only the rows in view, so a process list or a file tree of any size is safe.
- Deletion cannot be "just os.Remove": `mygo.Shell.TrashItem` is the only path that guarantees recoverability, and it fails instead of deleting when the item cannot be trashed.
- Mole's safety posture comes from three invariants worth copying: a path allow-list, a whitelist that disables cleanup when unreadable, and a fail-closed operation log.

## Requirements

**Functional**

- A window with a sidebar (Clean, Software, Startup, Optimize, Analyze, Status, Settings) and a route switch, no console window.
- A tray icon with Open/Quit, closing the window hides it and keeps the app alive.
- `safety.Guard.Check(path)` returning a typed refusal reason for protected paths, traversal, reparse points and UNC paths.
- Append-only operation log; reading a corrupt log fails explicitly.
- Settings store with atomic writes.
- English catalog wired through MyGo i18n so Vietnamese can be added in phase 7 without touching views.

**Non-functional**

- `internal/core/...` imports no MyGo package (enforced by test).
- Every guard decision is table-tested; no production path bypasses `safety.Guard`.
- `go vet ./...` clean; `go test ./...` fast (< 5 s).

## Architecture

Package layout created in this phase:

```
main.go                    app bootstrap: single instance, tray, window, view
internal/core/             pure Go, import boundary enforced
  safety/                  guard.go, roots.go, guard_test.go
  oplog/                   log.go, log_test.go
  settings/                settings.go, settings_test.go
internal/ui/               shell.go, routes.go, theme.go, spike.go (temporary)
locales/                   en.json + generated catalog_gen.go
resources/                 icon.png (1024²), tray.png (32²)
```

Guard semantics (the contract every later phase calls):

| Input | Result |
|---|---|
| Path outside every allow root | `RefuseOutsideRoot` |
| Volume root (`C:\`), `%SystemRoot%`, `%ProgramFiles%`, `%ProgramData%\Microsoft` | `RefuseProtected` |
| Profile root, Documents, Desktop, Downloads, Pictures, Videos, Music, OneDrive | `RefuseUserData` |
| Any ancestor with `FILE_ATTRIBUTE_REPARSE_POINT` (junction/symlink) | `RefuseReparse` |
| UNC path (`\\server\share`) | `RefuseUNC` |
| `..` that escapes after `filepath.Clean` | `RefuseOutsideRoot` |
| Alternate data stream (`name:stream`) | `RefuseADS` |
| Inside `%LOCALAPPDATA%`, `%APPDATA%`, `%TEMP%`, `%USERPROFILE%\.<dev cache>` | `Allow` |

Allow roots are computed once from `os.Getenv`/`KnownFolderPath`, compared after `filepath.Clean` and case folding, and never re-derived from user input.

## Related Code Files

Create:

- `go.mod`, `go.sum`, `.gitignore`
- `mygo.json` — name, identifier `dev.elom.app`, version `0.1.0`, `i18n: {dir: "locales", source: "en"}`
- `main.go` — single-instance lock, tray, window, hide-on-close, quit flag
- `resources/icon.png`, `resources/tray.png`
- `locales/en.json` — shell and safety messages
- `internal/core/doc.go`, `internal/core/arch_test.go` — import-boundary test
- `internal/core/safety/guard.go`, `roots.go`, `guard_test.go`
- `internal/core/oplog/log.go`, `log_test.go`
- `internal/core/settings/settings.go`, `settings_test.go`
- `internal/ui/shell.go`, `routes.go`, `theme.go`
- `internal/ui/spike.go` — temporary, deleted at the end of this phase
- `README.md`, `LICENSE` (MIT)

Modify: none (greenfield).

## Implementation Steps

1. `git init`; `go mod init elom`; add `.gitignore` (`build/`, `.mygo/`, `*.syso`).
2. `go get github.com/egoist/mygo@v0.8.3` and `go get -tool github.com/egoist/mygo/cmd/mygo@latest`; run `go tool mygo doctor` and record its output.
3. Write `mygo.json` and the two PNG icons; wire `locales` with `MyGo i18n` per https://mygo.egoist.dev/docs/i18n.md; commit the generated `catalog_gen.go`.
4. Implement `safety.Guard`: roots computed at first use (`sync.Once`), `Check` and `CheckAll` methods, typed errors with the refused path. Write a table test of ~30 cases including `C:\`, `C:\Windows\System32`, `\\?\C:\Windows`, `C:\Users\x\AppData\Local\..\..\Windows`, a junction in `%LOCALAPPDATA%`, and valid cache paths.
5. Implement `oplog` as JSONL at `App.Path(PathUserData)/operations.log`: `Record{RunID, Time, Op, Path, Bytes, Result, Err}`; `Append` fsync-free with buffered writer flushed per record; `Read` returns `ErrCorrupt` on a bad line. `RequireWritable()` returns an error when the file cannot be appended — callers refuse to proceed.
6. Implement `settings`: JSON at `App.Path(PathUserData)/settings.json`, atomic write (temp + `os.Rename`), `Whitelist() ([]string, error)` returning `ErrUnreadable` instead of an empty slice.
7. Build the shell: window 1180×760, min 960×640, `AutoHideMenuBar`; sidebar with the seven routes; footer showing reclaimable bytes placeholder; `WindowOptions{Content: ui.View(app.view)}`; tray menu Open/Quit; `OnClose` hides unless quitting (`OnBeforeQuit`).
8. Spike page (`internal/ui/spike.go`), each item exercised by hand and recorded:
   - `ui.Table` with 10 000 synthetic rows, column sort, and a row click;
   - `ui.Outline` over a fake file tree with 5 levels of indentation;
   - `ui.Box(...).Draw(...)` painting 200 treemap rectangles with `HitTest` reporting the hovered cell;
   - `chart.Plot` + `chart.Line` + `chart.MinMax` sparkline of 60 samples refreshing at 1 Hz;
   - tray icon with PNG menu icon, `Tray.PopUpMenu`, and a frameless `Panel` window positioned from `Tray.Bounds()` that hides on blur;
   - `GlobalShortcut.Register("Ctrl+Alt+E", …)`;
   - `mygo.Power.KeepAwake("spike", true)` releasing after 10 s.
9. Record the results, including anything that needed a workaround and any Windows 10 caveat, in `reports/spike-windows.md` inside this plan directory; delete `internal/ui/spike.go`.
10. Add `internal/core/arch_test.go`: run `go list -deps ./internal/core/...` from the module root and fail if any line contains `github.com/egoist/mygo`.
11. Write README (build/run/dev commands, safety summary) and MIT LICENSE.

## Todo

- [x] Module, tool directive, `.gitignore`, LICENSE, README
- [x] `mygo.json` + icons + locales wiring
- [x] `safety.Guard` with table tests
- [x] `oplog` with fail-closed read/write
- [x] `settings` with atomic write and `Whitelist()`
- [x] Shell window + sidebar + router + tray + hide-on-close
- [x] Spike: table, outline, draw+hit-test, chart, tray panel, shortcut, KeepAwake
- [x] `reports/spike-windows.md` written; spike view deleted
- [x] Import-boundary test
- [x] `go tool mygo build -platform windows/amd64` produces exe and installer

## Success Criteria

- [x] `go tool mygo dev` opens the shell; clicking sidebar items switches routes; no console window appears.
- [x] The tray icon shows the menu, and closing the window leaves the app in the tray.
- [x] `go test ./...` passes; `go vet ./...` reports nothing.
- [x] Guard table test covers all refusal reasons and passes.
- [x] Import-boundary test fails when a MyGo import is added to `internal/core` (verify once by hand, then revert).
- [x] `build/windows-amd64/elom.exe` and `elom-setup-0.1.0-amd64.exe` exist.
- [x] `reports/spike-windows.md` states, per spike item, observed behavior and any limitation.

## Verification

```powershell
go vet ./...
go test ./...
go tool mygo dev
go tool mygo build -platform windows/amd64
Test-Path build/windows-amd64/elom.exe, build/windows-amd64/elom-setup-0.1.0-amd64.exe
```

## Risk Assessment

| Risk | Mitigation |
|---|---|
| A widget needed later (e.g. per-row checkbox in a table) does not exist | Spike step 8 covers the shapes Clean/Analyze need; if `ui.Table` lacks composite cells, fall back to `ui.List` with a row builder |
| D3D11 renderer misbehaves on this machine | Note it; MyGo falls back to WARP. If broken, stop and report before writing features |
| Windows 10 differences unknown | No Windows 10 machine available — mark unverified in the report instead of guessing |
| MyGo API churn | Version pinned in `go.mod`; no API used beyond the documentation |

## Security Considerations

- The guard is the only gate for deletion and must be called by the deleting code path itself, not only by the UI.
- The oplog must never be silently degraded: an unwritable log blocks destructive actions in phase 2.
- No administrator rights are requested anywhere in this phase.

## Next Steps

Phase 2 adds the rule catalog, the scanner and the Clean screen on top of `safety` + `oplog`; no phase may re-implement path checks locally.
