# Phase 1 spike: MyGo native UI on Windows

Machine: Windows 11, amd64 (`go version go1.27.3 windows/amd64`), MyGo v0.8.3.
Date: 2026-11-10. Evidence: headless `ui.Tester` tests, a `-tags spike` build that
wrote its results to `%TEMP%\elom-spike.txt`, and a launched release build.

## Verified

| Item | How | Result |
|---|---|---|
| `ui.Table` with 10 000 rows | `ui.NewTester`, texts + click | Renders; row 0 and row 12 built, row 9999 **not** built (virtualized). Clicking "row 3" set selection to 3. |
| `ui.Outline` | `ui.NewTester` | Nested rows render (`C:\Windows\System32` visible when `C:\` and `C:\Windows` are in `OutlineState.Open`). `Open` has `Add`/`Remove`/`Clear`, not `Set`. |
| `Box(...).Draw` + `HitTest` | geometry unit test + headless render | `Painter.Fill(Rect, Color, radius)` and `Painter.Text(x, y, s, size, color)` work; `HitTest(func(x, y float32, r Rect) bool)` compiles and gates the pointer. Cell hit-testing verified as a pure function. |
| `chart.Plot` / `chart.Line` | headless render | Plot with `chart.Axes{Left: ticks}` and a `Draw` that strokes `chart.Line(r, xs, ys, chart.CurveLinear)` renders without error. |
| Tray icon | `-tags spike` build | `mygo.NewTray(TrayOptions{...})` returned a tray. |
| Tray flyout | `-tags spike` build | `mygo.NewFlyout(FlyoutOptions{Tray: tray, Width, Height, Content: ui.View(...)})` created. |
| Global shortcut | `-tags spike` build | `mygo.GlobalShortcut.Register("Ctrl+Alt+E", fn)` returned `nil` (registered; `RegisterHotKey`). |
| Keep screen on | `-tags spike` build | `mygo.Power.KeepAwake("Elom spike", true)` returned a release function. |
| App launch | release build, 5 s | Process alive, window title `Elom`, no crash. |
| Packaging | `go tool mygo build` | `build/windows-amd64/Elom.exe` 10.5 MB + `elom-setup-0.1.0-amd64.exe` 3.6 MB (NSIS, built automatically). |
| i18n | `go tool mygo generate`, `go tool mygo i18n check .` | 33 messages in `en`; accessors generated into `locales/catalog_gen.go`; `go test` passes with localized strings. |

## Corrections to the plan and the docs

- `WindowOptions` has `Panel` and `Content`, but the current API for a tray panel is
  `mygo.NewFlyout(FlyoutOptions{Tray: ...})`. Use the flyout in phase 4.
- There is no `VibrancyGlass` constant. Materials are `VibrancyWindow`, `VibrancyContent`,
  `VibrancySidebar`, `VibrancyHeader`, `VibrancyTitlebar`, `VibrancyUnderWindow`,
  `VibrancyUnderPage` plus transient ones (`VibrancyMenu`, `VibrancyPopover`, `VibrancyHUD`, …).
- `Element` has no `FontFamily`; monospace text comes from `ui.Font{Family: ...}` when shaping text
  for a `Painter`. Text elements use the theme's font.
- Widgets need an explicit size: a `Table` or `Box` inside a `Row` gets no height unless the
  container is `Fill()`. Without it the Tester reports “has no visible area”.
- `go mod init elom` + `go get -tool ...` bumped the `go` directive to 1.27.3 and Go downloaded
  that toolchain automatically; the machine had 1.27.0.

## Measured, and worth watching

- Working set after 5 s with the window open: **96.9 MB**. The plan's phase 7 target is ≤ 80 MB;
  the gap is likely the embedded i18n catalogs, images and the renderer's glyph atlas. Re-measure
  in phase 7 before deciding whether the target or the app needs work.
- Release binary 10.5 MB (vendor figure for a native-UI app is ~7 MB; ours embeds resources).

## Not verified (needs eyes on the screen)

- The tray icon's menu opens on click and reads `Open Elom` / `Quit`.
- Closing the window hides it and the app stays in the tray.
- The window has no console.
- Dark appearance, DPI scaling at 150%, and the sidebar's look.

These are visual checks for the next manual pass; nothing in the API surface blocks them.
