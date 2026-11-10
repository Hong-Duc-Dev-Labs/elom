---
phase: 4
title: "Live Status"
status: pending
priority: P2
effort: "1.5d"
dependencies: [1]
---

# Phase 4: Live Status

## Goal

Show live system health at 1 Hz — CPU, memory, disk I/O, network, uptime, top processes — with sparklines, a process table with End task, a tray flyout panel and a Keep Screen On switch.

## Context Links

- Shell and tray plumbing: [phase-01-spike-skeleton.md](./phase-01-spike-skeleton.md)
- MyGo power API: https://mygo.egoist.dev/docs/native.md#power
- Tray behavior: https://mygo.egoist.dev/docs/menus.md#tray-icons
- gopsutil Windows support: https://github.com/shirou/gopsutil (README status matrix)

## Key Insights

- gopsutil needs no cgo and covers Windows amd64/arm64 for cpu, mem, disk, net, process — verified in its README status table. It is the only metrics dependency needed.
- `chart.MinMax` keeps one low and one high sample per pixel column, so a 60-sample ring draws without allocating and without losing spikes.
- The tray flyout is a `Panel` window positioned from `Tray.Bounds()`; `OnClick` on the tray toggles it, `OnBlur` hides it, exactly as `examples/menubar-native` does.
- `mygo.Power.KeepAwake(reason, display)` is the whole "Keep Screen On" feature — no `SetThreadExecutionState` syscall needed.

## Requirements

**Functional**

- Sampler at 1 Hz: CPU total and per-core, memory used/available, disk read/write bytes per second, network receive/send per second, uptime, battery state when present.
- 60-sample ring per metric; sparklines under each card; a health score derived from the same samples with a documented formula.
- Process table: top 25 by CPU, columns name, PID, CPU %, memory, user; sortable; refresh 2 s; "End task" with confirmation.
- Tray: icon tooltip with CPU %; click opens a 360×300 frameless panel with the same numbers plus top 3 processes; `Ctrl+Alt+E` toggles it; `Esc` hides it.
- Keep Screen On: switch in the tray panel and Status page, `Power.KeepAwake("Elom: Keep screen on", true)` held until switched off or app exit; state shown in the tray tooltip.

**Non-functional**

- Idle CPU below 1% with the main window hidden and the sampler running.
- Sampling never blocks the UI thread; the view reads an immutable snapshot.
- Process enumeration cost stays under 150 ms per tick.

## Architecture

```
metrics.Sampler (goroutine, 1 Hz)
   ├─ cpu.Percent, mem.VirtualMemory, disk.IOCounters, net.IOCounters, host.Uptime
   ├─ ring.Ring[float64] (cap 60) per metric
   └─ Snapshot{Metrics, History, Processes, At}
             │  (mutex-protected pointer swap)
             ▼
   ui.status view  ── reads snapshot ──► cards, sparklines, table
   ui.traypanel    ── same snapshot via App-level store ──► compact view
```

`Ring.T` is written only by the sampler goroutine; readers take a copy under a mutex or an atomic pointer to an immutable snapshot, so no per-frame allocation occurs in the view.

Health score: `100 - (cpuPenalty + memPenalty + diskPenalty)`, where each penalty is `min(30, value)` for CPU % ≥ 85 or memory ≥ 90% conditions. The formula is displayed in a tooltip so the number is not magic.

## Related Code Files

Create:

- `internal/core/metrics/sampler.go`, `ring.go`, `processes.go`, `sampler_test.go`, `ring_test.go`
- `internal/svc/status.go` — sampler lifecycle tied to app ready/quit
- `internal/ui/status.go` — cards, sparklines, table, End task
- `internal/ui/traypanel.go` — flyout panel, Keep Screen On switch

Modify:

- `main.go` — start/stop sampler, tray icon click, panel window, global shortcut
- `locales/en.json` — Status strings

## Implementation Steps

1. `metrics.Ring`: fixed-capacity float64 ring with `Push`, `Values(dst []float64) []float64`, `Len`; tests for wraparound and no-allocation reuse.
2. `metrics.Sampler`: constructor with interval and clock injection; `Snapshot()` returns a value copy; per-tick collection with independent error handling (a failing metric logs once and yields zeros, it does not stop the sampler).
3. Process sampling: `process.Processes`, per-process `CPUPercent`/`MemoryInfo`, stable sort by CPU desc, cap 100 for the snapshot; measure tick cost on this machine and record it.
4. Status view: six cards in a responsive grid, each with a big number, a unit, and a 60-sample sparkline (`chart.Plot` + `chart.Line` + `chart.MinMax`), plus a legend explaining the metric; values formatted with `locales` and the user's region.
5. Process table with `ui.Table`: sortable headers, selection, End task through `process.Terminate` after a confirmation that names the process and warns about unsaved work; refresh every 2 s without losing scroll position.
6. Tray panel: `WindowOptions{Panel: true, Frameless: true, Transparent: true, Hidden: true, Vibrancy: mygo.VibrancyGlass, CornerRadius: 12, Width: 360, Height: 300, DisableResize: true}`; position from `Tray.Bounds()` clamped to the work area; `OnBlur` hides; `OnClose` prevents default and hides.
7. Tray icon: `ToolTip` updated at most once per second with CPU %; menu with Open, Keep Screen On (checkbox), Quit; `PopUpMenu` only on right click by setting the menu inside `OnRightClick` and clearing it (documented pattern).
8. Keep Screen On: hold the release function in app state, release on switch-off and on quit; reflect the state in the tray menu checkbox and the panel switch.
9. Shortcut `Ctrl+Alt+E` toggles the panel; report a registration failure in Settings rather than failing silently.

## Todo

- [x] Ring buffer + tests
- [x] Sampler with injected interval and per-metric error isolation
- [x] Process sampling with measured tick cost
- [x] Status cards + sparklines + health score
- [ ] Health score tooltip explaining the formula
- [x] Process table + End task confirmation
- [ ] Tray tooltip showing live CPU %
- [x] Tray icon, right-click menu, flyout panel
- [x] Global shortcut registration + failure reporting
- [x] Keep Screen On via `Power.KeepAwake`

## Evidence

Measured on this machine (`ELOM_REAL_SAMPLE=1`):

| Reading | Value |
|---|---|
| One `Read` through gopsutil | 18–29 ms |
| Processes in the table (top by CPU) | 40 |
| App idle CPU over 10 s, window open, sampler running | **0.18 %** |
| Working set with sampler | 72.8 MB |

Automated (`go test ./...`):

- Ring wraps and reuses its buffer; sparkline history is oldest-first.
- Rates come from counter deltas; a counter that goes backwards reports 0, not a spike.
- A failing read keeps the previous snapshot instead of crashing the sampler.
- Health score table (100 at idle, 45 at full saturation).
- Status page and panel render values and processes from a fabricated snapshot.

Replaced the plan's `Panel` window with `mygo.NewFlyout(FlyoutOptions{Tray: ...})`, which anchors
itself to the tray icon and hides when it loses focus, so positioning code is not needed.

Needs a manual pass:

- Sparkline look and the tray flyout on screen (rendered headlessly only).
- The tray tooltip does not show CPU % yet, and the health score has no tooltip;
  both are follow-ups rather than blockers.

## Success Criteria

- [ ] Metrics update every second and match Task Manager within ±5% for CPU and memory over a 60 s sample.
- [ ] Sparklines show 60 samples and redraw only where data changed.
- [ ] The tray panel opens from the icon and from `Ctrl+Alt+E`, and hides on blur or `Esc`.
- [ ] Idle CPU stays below 1% for 60 s with the main window hidden.
- [ ] End task terminates a test process (`notepad.exe`) after confirmation and logs nothing to the oplog (process kill is not a file deletion).
- [ ] Keep Screen On holds the display awake for 10 minutes (verified with the display timeout setting) and releases on quit.

## Verification

```powershell
go test ./internal/core/metrics/...
go tool mygo dev
# idle CPU check for 60s:
(Get-Counter '\Process(elom)\% Processor Time' -SampleInterval 1 -MaxSamples 60).CounterSamples |
  Measure-Object CookedValue -Average
```

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Process enumeration is expensive on many-core machines | Measure per tick; sample processes every 2 s while system metrics stay at 1 s |
| CPU % requires two samples, first tick reads 0 | Prime the sampler once at start and discard the first value |
| Panel steals focus or fails to hide | Use the documented panel flow (create hidden, `OnBlur` hide, `OnClose` preventDefault) and test both tray click and shortcut |
| Sparkline allocation per frame | Reuse slices in view state with `chart.MinMax` writing into them |

## Security Considerations

- End task is the only process-termination path and always requires a confirmation naming the process.
- No administrator rights; terminating a protected process fails and surfaces the OS error.
- No telemetry: samples stay in memory; nothing is written to disk.

## Next Steps

Phase 5 adds installed-app enumeration and startup control; both reuse the guarded delete path for leftovers.
