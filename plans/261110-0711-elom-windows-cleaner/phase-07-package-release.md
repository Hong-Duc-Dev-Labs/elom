---
phase: 7
title: "Package & Release"
status: pending
priority: P2
effort: "1d"
dependencies: [1, 2, 3, 4, 5, 6]
---

# Phase 7: Package & Release

## Goal

Turn the working app into a shippable one: settings and safety surfaces (whitelist, oplog viewer, Clean Screen), Vietnamese localization, installer and update configuration, and an executed acceptance sweep with recorded evidence.

## Context Links

- Acceptance criteria: [plan.md](./plan.md#success-criteria)
- MyGo distribution and installer: https://mygo.egoist.dev/docs/distribution.md
- MyGo updater: https://mygo.egoist.dev/docs/plugins/updater.md
- i18n catalogs: https://mygo.egoist.dev/docs/i18n.md

## Key Insights

- The NSIS installer MyGo generates installs per user in `%LOCALAPPDATA%\Programs` with no administrator rights, and `mygo build` produces `windows.installerInclude` hooks for anything extra — neither is needed to ship a first version.
- Updates are signed with a key generated for the app; without a published release host the updater is inert, so configuration lands now but activation waits for a repository.
- A cleaner's credibility rests on what it refuses to do: the whitelist editor, the oplog viewer and the "never touched" list belong in the shipped UI, not only in docs.
- Clean Screen is `Power.KeepAwake`-independent: a topmost fullscreen window with a solid color, blocked input, `Esc` to exit — small, but it must not swallow `Esc`.

## Requirements

**Functional**

- Settings page: permanent-delete toggle with warning, scan limits (depth, entry cap), maximum scan age, whitelist editor (globs with validation), oplog viewer (last 500 records, filter by op, reveal log file), language selector (System / English / Tiếng Việt).
- Clean Screen: fullscreen topmost window on the display with the pointer, solid color, hides on `Esc` or click-to-exit after 2 s, optional keyboard lock.
- Vietnamese catalog covering every user-visible string; language switch applies immediately.
- Installer: name Elom, icon, Start menu shortcut, uninstall entry, URL scheme none.
- Updater: configured with a generated signing key and a release feed placeholder; UI shows "updates not configured" until a feed exists.
- `docs/verification.md` with every acceptance criterion, the command that proves it, and the observed result on a named machine/date.
- `SECURITY.md` stating what is never touched and how to report an issue.

**Non-functional**

- Installed app size ≤ 25 MB; memory at idle ≤ 80 MB with the window open.
- No network access except the updater check.
- `README.md` covers build, dev, test, packaging and the safety model.

## Architecture

```
build/windows-amd64/
  elom.exe                     the app (native UI, no webview)
  elom-setup-1.0.0-amd64.exe   NSIS per-user installer
  install.ps1                  when updates are configured
```

Settings surface reads/writes through `internal/core/settings`; the whitelist editor validates globs with the same matcher the scanner uses (`rules.MatchSegment`), so an invalid pattern is rejected at the source.

## Related Code Files

Create:

- `locales/vi.json`
- `internal/ui/settings.go` — toggles, whitelist editor, oplog viewer, language selector
- `internal/ui/cleanscreen.go` — fullscreen cover window
- `docs/verification.md`, `SECURITY.md`
- `build/installer.nsh` — only if a shortcut or file association beyond MyGo's defaults is needed

Modify:

- `mygo.json` — version `1.0.0`, updater keys, `windows.displayName`
- `README.md`, `locales/en.json`
- `main.go` — route for Settings, Clean Screen window, language menu

## Implementation Steps

1. Settings page implementation with immediate-apply toggles, whitelist editor (add/remove pattern, inline validation, "this protects X items" preview), and the oplog viewer reading `oplog.Read` with filters.
2. Clean Screen window: `WindowOptions{Frameless: true, Topmost: true, DisableResize: true, Content: ui.View(...)}` covering the pointer's display work area, solid background, `Esc` and a 2 s click guard to exit; optional input lock with a visible hint.
3. Vietnamese catalog: translate every key, keep placeholders identical, run `go tool mygo generate` and check `src` parity with English keys; add the language menu using `mygo.App.Languages()` and `mygo.App.SetLocale`.
4. Updater: generate the update signing keypair, add public key to `mygo.json`, configure the release feed; verify `mygo build` produces signed update artifacts; the UI shows a clear state when the feed is unreachable.
5. Packaging: `go tool mygo build -platform windows/amd64,windows/arm64`; install the installer output, verify Start menu entry, uninstall entry, and clean removal including settings (ask before deleting `%APPDATA%\elom`).
6. Write `docs/verification.md` and execute it top to bottom, recording evidence (command output, screenshots of key screens in `docs/img/`), including the unverified Windows 10 note if no machine is available.
7. Write `SECURITY.md`: protected locations, how elevation works, what the oplog contains, how to report a vulnerability.
8. Update README with the feature list, the safety model, the acceptance commands and the module layout; add the measured scan timing from phase 3.
9. Release checklist in `plan.md` (status update via `ak plan phase`) and a final `go vet ./... && go test ./...` run.

## Todo

- [x] Settings page (toggles, whitelist editor, oplog viewer, language)
- [x] Clean Screen window with `Esc` exit and a 2-second click guard
- [x] Vietnamese catalog complete and parity-tested
- [ ] Updater key + feed configuration (needs a release host)
- [x] Installer install/uninstall verification
- [x] `docs/verification.md` executed with evidence
- [x] `SECURITY.md` and README finalized
- [x] Full test + build sweep

## Evidence

| Check | Result |
|---|---|
| Catalogs | **128 messages** in `en` and `vi`; `mygo i18n check` agrees; the parity test fails on a missing key or a changed placeholder |
| Silent install `/S` | installs to `%LOCALAPPDATA%\Programs\Elom` without a UAC prompt (`Elom.exe`, icons, `Uninstall.exe`) |
| Silent uninstall `/S` | removes the install directory entirely (`Test-Path` false afterwards) |
| Suite | 15 packages green, `go vet` clean |
| Build | `Elom.exe` 11.5 MB, installer 3.9 MB |

Clean Screen opens its own frameless, always-on-top, full-screen window; `Esc`
hides it, and a click does after two seconds so a wipe cannot dismiss it by
accident.

Not done: the updater is not configured — it needs a signing key and a release
feed, and shipping a key without a host would only look finished.

## Success Criteria

- [ ] The installer installs for the current user without a UAC prompt and uninstalls without leaving files behind (except user data, which it offers to remove).
- [ ] Switching to Tiếng Việt changes every screen and menu label with no missing-key fallback visible.
- [ ] Clean Screen covers the display, blocks accidental clicks for 2 s, and exits on `Esc`.
- [ ] Whitelist entries protect their paths in Clean, Analyze and Software, verified end to end by protecting one cache folder and confirming it disappears from results.
- [ ] `docs/verification.md` records all nine acceptance criteria as passed, failed, or explicitly unverified with a reason.
- [ ] `go vet ./...` and `go test ./...` pass on a clean checkout.
- [ ] Installed size and idle memory meet the stated non-functional targets.

## Verification

```powershell
go vet ./...
go test ./...
go tool mygo build -platform windows/amd64,windows/arm64
Start-Process build\windows-amd64\elom-setup-1.0.0-amd64.exe
Get-ChildItem "$env:LOCALAPPDATA\Programs\Elom"
Get-Content "$env:APPDATA\elom\operations.log" -Tail 20
```

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Unsigned binary triggers SmartScreen on other machines | Document the warning; keep `windows.certificate` ready; publish SHA-256 checksums with the release |
| Vietnamese plural/case strings break ICU parsing | Keep placeholders identical to English; `mygo generate` fails loudly on bad ICU, run it in the verification step |
| Uninstaller removes user data unexpectedly | Never delete `%APPDATA%\elom` during uninstall; offer it in the uninstall UI only |
| Updater misconfigured and silently inert | UI states "updates not configured" until a feed answers; no background retry loop |
| Clean Screen traps the user | `Esc` always exits, click exits after 2 s, and the hint is drawn on the overlay |

## Security Considerations

- Update artifacts are signed and verified against the embedded public key; a failed signature check aborts.
- The whitelist is fail-closed: an unreadable or invalid whitelist file blocks destructive actions.
- `SECURITY.md` documents the threat model: elevation is per-task, the job file is validated, and the app performs no network requests except update checks.

## Next Steps

Execution: `/ak-cook D:\Dev\elom\plans\261110-0711-elom-windows-cleaner\plan.md`. Start with phase 1; the spike report gates the rest.
