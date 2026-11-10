---
title: "Elom plan: Mole-for-Windows clone on MyGo"
date: 2026-11-10
summary: Brainstorm locked native MyGo UI and full parity; plan scaffolds 7 phases and 108 tasks for a Windows cleaner.
---

# Elom plan: Mole-for-Windows clone on MyGo

## What happened

Turned an empty directory into a planned product. Researched Mole for Windows (mole.fit/windows, tw93/Mole) and MyGo v0.8.3 (docs plus a local clone of github.com/egoist/mygo), then wrote a seven-phase plan for `elom`, a native MyGo clone of Mole.

Verified against upstream source, not memory: `mygo.NewTray/TrayOptions`, `Tray.Bounds/PopUpMenu`, `GlobalShortcut.Register`, `Shell.TrashItem/FileIcon`, `Power.KeepAwake(reason, display)`, `ui.Table/Outline/Sidebar/Split`, `Element.Draw/HitTest`, `ui/chart.Plot/Line/MinMax`, `ui.NewTester`, and `mygo.json` i18n catalogs. gopsutil v4 states all work is implemented without cgo, which satisfies the no-cgo constraint.

## Decisions

- Native MyGo UI (no webview) over a React frontend: ~7 MB binary, no WebView2 dependency on Windows 10, and every needed widget exists.
- Full feature parity with Mole for Windows, delivered in seven sequential phases; free and open source, MIT, no GPL code copied from the Mole CLI.
- Safety core first: `internal/core/safety` guard with a typed refusal table, fail-closed operation log, Recycle Bin default, and an import-boundary test that keeps `internal/core` free of MyGo imports so a CLI can reuse it later.
- Privileged work runs in a second, short-lived instance of the same binary driven by a validated job file, never by elevating the UI.

## Verification

`ak plan validate` passes; `ak plan parse` reports 7 phases and 108 tasks. Scanned the plan for stale names, missing links and version drift (installer version, `requiresAdmin` rule field, unelevated DNS flush).

## Next steps

Execute phase 1 (spike + skeleton) and record widget behavior in `reports/spike-windows.md`, then Clean, Analyze, Status, Apps/Startup, Optimize/elevation, packaging.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
