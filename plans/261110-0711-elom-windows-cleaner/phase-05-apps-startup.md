---
phase: 5
title: "Apps & Startup"
status: pending
priority: P1
effort: "2.5d"
dependencies: [1, 2]
---

# Phase 5: Apps & Startup

## Goal

Deliver the Software and Startup modules: enumerate installed apps (registry and packaged), run their own uninstallers, review leftovers before deletion, list winget updates, and control startup entries with state the OS respects.

## Context Links

- Guard/oplog/delete path: [phase-02-clean-engine.md](./phase-02-clean-engine.md)
- Mole CLI uninstall behavior and dry-run wording: https://github.com/tw93/Mole/tree/windows#readme
- Startup internals reference (registry + StartupApproved): https://mole.fit/windows/blog/manage-windows-startup
- MyGo shell helpers (icons, reveal): https://mygo.egoist.dev/docs/native.md#the-shell

## Key Insights

- Installed apps live in up to six registry locations (HKLM/HKCU × 32/64-bit view); missing the WOW6432Node view is the classic cause of "my uninstaller shows half the apps".
- Leftover matching is heuristic by nature. Mole's answer is confidence levels plus opt-in: nothing is checked by default, and personal data folders are excluded from matching.
- UWP/packaged apps need `Get-AppxPackage`; emit JSON (`ConvertTo-Json -Compress`) instead of parsing a table, and remove with `Remove-AppxPackage` only after the user selects the entry.
- StartupApproved's blob is what Task Manager's Startup tab actually writes; toggling only `Run` keys desynchronizes Task Manager. Verify byte layout empirically before writing it.
- `winget upgrade` prints a localized table; a tolerant parser plus a "manual refresh" affordance is honest, and no update is applied without an explicit selection.

## Requirements

**Functional**

- Software list: name, version, publisher, install location, estimated size, source (registry / packaged), icon; sorted by size desc; search field.
- Uninstall: run `QuietUninstallString` or `UninstallString` with a visible progress state, capture exit code, then scan leftovers.
- Leftover review: candidate paths with size, confidence (`high`/`medium`), reason; all unchecked; personal folders (`Desktop`, `Documents`, `Downloads`, `OneDrive`) never proposed.
- Updates: list `winget upgrade` entries (name, id, current, available); apply one at a time with streamed output; never batch-apply silently.
- Startup list: Run/RunOnce (HKCU/HKLM), startup folders, StartupApproved state, scheduled tasks (read-only, with a "managed by Task Scheduler" note).
- Startup toggle: enabled/disabled written to StartupApproved (HKCU without elevation, HKLM via the phase 6 job protocol), with revert.

**Non-functional**

- App enumeration ≤ 2 s warm; icons loaded lazily per visible row and cached by path.
- Leftover scan bounded: only `%LOCALAPPDATA%`, `%APPDATA%`, `%PROGRAMDATA%`, `%LOCALAPPDATA%\Programs` and the recorded `InstallLocation`.
- Every leftover deletion goes through guard + oplog; uninstaller output is shown, not swallowed.

## Architecture

```
apps.Enumerate() ──► []App{Name, Version, Publisher, InstallLocation, SizeKB, Kind, Uninstall, Quiet}
      ├─ registry: HKLM/HKCU, 64- and 32-bit views, Uninstall subkeys
      └─ packaged: powershell Get-AppxPackage | Select | ConvertTo-Json

apps.Leftovers(app) ──► []Leftover{Path, Bytes, Confidence, Reason, Checked:false}

startup.Enumerate() ──► []Entry{Name, Command, Source, Enabled, Scope, Writable}
      ├─ Run / RunOnce keys
      ├─ Startup folders
      └─ StartupApproved\Run (enabled state)

winget.Client ──► List() []Upgrade{ID, Name, Current, Available}
                 Upgrade(id, onLine func(string)) error
```

## Related Code Files

Create:

- `internal/core/apps/registry.go`, `packaged.go`, `leftovers.go`, `uninstall.go`, `registry_test.go`, `leftovers_test.go`
- `internal/core/startup/run.go`, `approved.go`, `folder.go`, `startup_test.go`
- `internal/core/winget/client.go`, `parse.go`, `parse_test.go`, `testdata/upgrade_en.txt`
- `internal/svc/apps.go`, `internal/svc/startup.go`
- `internal/ui/apps.go`, `internal/ui/startup.go`

Modify:

- `main.go` — mount Software and Startup routes
- `locales/en.json` — new strings
- `internal/core/settings/settings.go` — remember "show packaged apps" and last Software sort

## Implementation Steps

1. Registry enumeration: open `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall` with `registry.WOW64_64KEY` and `WOW64_32KEY` for HKLM and HKCU; skip entries with `SystemComponent=1`, `ParentKeyName`, or no `DisplayName`; read `DisplayName`, `DisplayVersion`, `Publisher`, `InstallLocation`, `UninstallString`, `QuietUninstallString`, `EstimatedSize`, `DisplayIcon`.
2. Packaged apps: `powershell.exe -NoProfile -NonInteractive -Command "Get-AppxPackage | Select-Object Name,PackageFullName,PublisherDisplayName,InstallLocation | ConvertTo-Json -Compress"`; parse into the same `App` type with `Kind: "appx"`; hide framework packages by name pattern.
3. Unit tests for registry parsing (fixture-backed by an injectable reader), name normalization, and the appx JSON decoder including single-object and empty results.
4. Leftover scanner: tokens from app name and publisher (strip "Inc.", "Ltd", version numbers); candidates under the bounded roots; confidence `high` when the directory name contains the full normalized app name, `medium` for publisher-only matches; exclude allow-listed data folders; require size > 0.
5. Leftover tests: a synthetic profile fixture with owning, ambiguous and unrelated folders; assert personal folders never appear and that medium matches default to unchecked.
6. Uninstall runner: `cmd /c <UninstallString>` with `HideWindow`, streamed stdout/stderr into the progress panel (bounded buffer), 15 min timeout, exit code captured; refuse when the command is empty and say why.
7. Software UI: searchable table, detail pane (icon via `Shell.FileIcon` cached, publisher, location, "Reveal in Explorer"), Uninstall button → progress → leftovers step with checkboxes and totals → confirm → trash leftovers + oplog.
8. Winget client: `winget upgrade --include-unknown --disable-interactivity --accept-source-agreements` capture; tolerant parser (fixed column positions computed from the header line, skip rules/separators); fixture tests for English output and a "cannot parse" path that surfaces a manual-check message instead of empty results.
9. Winget apply: `winget upgrade --id <id> --silent --accept-package-agreements --accept-source-agreements`, line-streamed progress, refresh list on completion; one at a time.
10. Startup enumeration and toggle: read Run keys and folders; read `StartupApproved\Run` blobs and decode enabled state; verify the byte layout by toggling one entry in Task Manager, dumping the blob before/after, and recording both in a test fixture; write enabled/disabled for HKCU; route HKLM through a job op in phase 6.
11. Startup UI: table of entries with source badge, enabled toggle, command preview, "Open in registry" (`regedit /e`? no — `Shell.ShowItemInFolder` on the executable path) and "Disable" writing the approved flag; revert restores the original bytes.

## Todo

- [x] Registry enumeration across both views and hives
- [x] Packaged app enumeration via `ConvertTo-Json`
- [x] App normalization + fixture tests
- [x] Leftover scanner with confidence levels and exclusions
- [x] Uninstall runner with streamed output and a 15-minute timeout
- [x] Software detail pane and icon cache (`Shell.FileIcon`, cached per path)
- [x] Winget list parser + fixtures + apply
- [x] Startup enumeration (Run, folders, StartupApproved)
- [ ] StartupApproved byte layout verified against Task Manager (not yet toggled on a real entry)
- [x] Startup UI with revert

## Evidence

Measured on this machine:

| Probe | Result |
|---|---|
| Registry apps (`apps.Enumerate`) | **366** |
| Packaged apps (`Get-AppxPackage`) | **131** |
| Startup entries | **17**, with real enabled flags (Discord disabled, Chrome enabled) |
| `winget upgrade` parsed | **29** updates in 5.4 s |

Automated (`go test ./...`): 13 packages green.

- `ParseUninstall` drops components, updates and nameless keys; quiet uninstall preferred.
- `NormalizeName` strips versions/publisher suffixes; folder matching returns high/medium/none.
- `FindLeftovers` proposes only matching folders, honours the exclusion callback.
- `ParseUpgrade` reads the English table by header position, returns empty for "nothing to update",
  and returns `ErrUnparseable` instead of guessing on localized output.
- Startup blob encode/decode round-trips; Software and Startup screens render from fixtures,
  including search filtering and the unparseable-winget state.
- Detail pane shows publisher, location and Reveal; icons are read once per path and cached
  (`iconPath` strips the `,0` index some uninstallers append).

Needs a manual pass:

- Run one real uninstaller and confirm the leftovers list afterwards.
- Toggle a startup entry, reboot, and compare with Task Manager.
- Apply one winget update and confirm the version afterwards.

## Success Criteria

- [ ] App count is within 5% of Settings › Apps on this machine, and both a registry app and a packaged app appear.
- [ ] Uninstalling one test app runs its own uninstaller, captures the exit code, and afterwards proposes leftovers unchecked.
- [ ] Deleting selected leftovers moves them to the Recycle Bin and restores them.
- [ ] Winget list matches `winget upgrade` output for at least three entries; an unparseable output shows an explicit message, not an empty list.
- [ ] Startup list matches Task Manager's Startup tab names; toggling one entry to disabled survives a reboot and Task Manager shows it disabled too; revert restores the exact original bytes.
- [ ] Scheduled tasks appear read-only with a note explaining why they are not toggled here.

## Verification

```powershell
go test ./internal/core/apps/... ./internal/core/startup/... ./internal/core/winget/...
go tool mygo dev
(Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*').Count
winget upgrade --include-unknown
Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run'
```

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Uninstaller requires elevation or a GUI | Detect and route to the elevated job (phase 6) or let the vendor UI run; report the blocked case instead of forcing deletion |
| Leftover heuristic deletes user data | Unchecked by default, confidence shown, personal folders excluded, guard + Recycle Bin |
| StartupApproved bytes differ across Windows versions | Fixture captured on this machine; write only the flag byte, preserve the rest; revert from a saved copy |
| winget output changes or is localized | Header-position parser + fixture tests; failure surfaces a message; no silent "no updates" |
| Appx enumeration is slow | Cache for 60 s; refresh button; run off the UI thread |

## Security Considerations

- Vendor uninstallers are executed as-is; the app never fabricates uninstall commands.
- Command strings come from the registry of the machine and are passed to `cmd /c` verbatim — they are not user-editable in the UI.
- Leftover deletion always passes `safety.Guard`; `%PROGRAMDATA%` entries require explicit user selection and are never default-checked.
- HKLM startup writes are deferred to the validated elevated job, never written directly.

## Next Steps

Phase 6 provides the elevated job protocol that this phase already routes privileged actions to.
