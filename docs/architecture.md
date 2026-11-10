# Architecture

Elom is one Go process that draws a native window with
[MyGo](https://mygo.egoist.dev). There is no webview, no cgo and no JavaScript.
This page describes the layers, the screen model, the design system and the
places where a change belongs.

## Layers

```
main.go                 single-instance window; elevated-job mode
  └─ internal/ui        native widgets, screens, UI-thread state
       └─ internal/svc  shell and OS integration used by the UI
            └─ internal/core   pure Go domain logic (no MyGo imports)
locales/                en + vi catalogs and generated accessors
```

The dependency direction is one-way. `internal/core/arch_test.go` fails the
build if core imports MyGo, which keeps scanning, guarding, logging and parsing
testable without a window.

| Package | Responsibility |
|---|---|
| `core/safety` | refuses roots, system folders, personal folders, UNC, ADS, junctions and links |
| `core/oplog` | append-only operation records with run ids and outcomes |
| `core/settings` | preferences and the protection list |
| `core/rules` + `core/scan` | cleanup catalog, expansion and candidate sizing |
| `core/delete` | Recycle Bin and permanent deletion, with a second guard check |
| `core/apps` | installed apps, leftovers and the vendor uninstall runner |
| `core/startup` | Run keys, `StartupApproved`, Startup folders |
| `core/winget` | update listing and the tolerant table parser |
| `core/maint` | the closed table of maintenance operations and job files |
| `core/metrics` | the 1 Hz sampler, history rings and process list |
| `svc` | adapters the UI calls: clean, software, startup, status, elevation |
| `ui` | `App.View`, navigation, screens, shared design components |

## Screens

Navigation lives in [`internal/ui/routes.go`](../internal/ui/routes.go).

| Screen | File | Shape |
|---|---|---|
| Clean | `clean.go` | hero result → review list → confirm → result |
| Software | `apps.go`, `software.go` | own tab strip: Uninstall, Updates, Startup |
| Startup | `startup.go` | list of entries with toggles and scope labels |
| Optimize | `optimize.go` | hero, routine action, task list with per-task results |
| Analyze | `analyze.go`, `treemap.go` | breadcrumb, treemap/largest switch, detail card |
| Status | `status.go`, `planet.go` | health tile, resource tiles, process table |
| Settings | `shell.go` (`settingsPage`) | two-column card of rows and lists |

Shell chrome (`topBar`, nav pill, `page`, `heading`, `framePalette`) is in
`shell.go`. Clean and Analyze are full-height screens with their own scrolling
lists; the rest scroll as a page.

## Design system

The UI follows Mole's measured design language; see
[`ATTRIBUTION.md`](../ATTRIBUTION.md) for the precise relationship and
licensing note. Tokens and components live in
[`internal/ui/design.go`](../internal/ui/design.go).

- **Ground:** base `#15141b`; each screen has a palette (`paletteFor`) whose
  wash tints the window with Mole's alpha ramp (217/255 at the top, 107/255
  from the middle down). Cards are that wash composited over the base and sunk
  15 % back (`cardGround`). The tab switch cross-fades only the wash, glow and
  ink.
- **Type:** one scale (`sizeFootnote` 10, `sizeCaption` 11, `sizeMeta` 11.5,
  `sizeMicro` 10.5, `sizeBody` 13.5, `sizeValue` 24). Small capitals always go
  through `capsLabel` (Geist Mono 9.5, tracking 1.1).
- **Components:** `card`/`cardWith`, `cardHead` (dot, tracked label, meta),
  `rule`, `listRow` (44 DIP, radius 8), `rowDivider` (inset 8), `pillTag`,
  `toggle`, `bar`, `bigNumber`, `hintText`, `primaryPill`, `ghostPill`, and the
  chart family. Screens should compose these rather than re-declare sizes,
  colours or padding.
- **Text roles:** `fgPrimary`, `fgSecondary` (72 %), `fgMuted` (62 %),
  `fgFaint` (52 %). `design_test.go` pins the primary/secondary/muted contrast
  floors per screen.

## State and threading

`App` is the single view model. The view is rebuilt from it on the main thread,
so a screen reads state; it does not own DOM or widget state.

- `App.update(fn)` runs `fn` on the UI thread, or directly when no window
  exists (tests). Background work (scan, index, uninstall, metrics) delivers
  results through it.
- Long jobs keep their state next to the screen (`cleanState`, `softwareState`,
  `optimizeState`, …) and expose progress, cancellation and errors as fields
  the view renders. Errors are stored raw and wrapped by the catalog at render
  time.
- The metrics sampler writes `App.status` at 1 Hz; the screen renders that
  snapshot.

## Deletion and privilege boundary

Cleanup is review-first: a scan produces candidates with a reason, the user
selects, and only then is anything removed. The safety guard runs during the
scan and again at deletion. The protection list and the operation log failing
to read stops cleanup.

The UI never runs elevated. Privileged tasks are listed in the closed table in
`core/maint`, written to a job file, executed by a second invocation of the
same executable through `ShellExecuteW("runas", …)`, re-validated there, and
their results are read back. See [`SECURITY.md`](../SECURITY.md).

## Messages

`locales/en.json` is the source catalog and `locales/vi.json` its translation.
`go tool mygo generate` writes `locales/catalog_gen.go`; do not edit the
generated file. Code asks for messages through the generated accessors, e.g.
`locales.StatusKillFailed(name, reason)`, so argument shapes are checked by
`go tool mygo i18n check .`.

## Tests

| Layer | Where | What it proves |
|---|---|---|
| Core tables and parsers | `internal/core/*_test.go` | guard cases, cleaning rules, parsers, job validation, ring buffers |
| Services | `internal/svc/*_test.go` | scan/index/uninstall orchestration with fakes |
| Headless view | `internal/ui/*_test.go` | `NewTester`/`Render` read the actual widget tree: labels, states, accessibility names, layout fit |
| Visual | `TestGallery` (`ELOM_GALLERY=1`) | one PNG per screen for a human/agent visual pass |
| Real-machine probes | opt-in env vars | see [`verification.md`](verification.md) |

A behavioural fix should come with a headless view test or a core/service test
that fails without it. Prefer the narrowest assertion that captures the
contract (a visible message, a state transition, a routed operation), not a
copy of the implementation.

## Where to change what

- New message: `locales/en.json` + `locales/vi.json`, then regenerate.
- New screen: add it to `routes.go`, give it a palette, render it through
  `page`, and add a gallery fixture.
- New privileged operation: add it to `core/maint.Tasks`, implement it in
  `core/maint/run_*.go`, and surface it through a service or Optimize.
- New deletion path: call `core/delete` (never `os.Remove` directly) so the
  guard and log stay in force.
- New shared look: add a component to `design.go` and migrate the call sites;
  do not fork a private variant in one screen.
