# Feature Comparison: Mole for Windows UI → Elom

Mode: `--compare` + `--port` analysis. Date: 2026-11-10.

## Source manifest

| Field | Value |
|---|---|
| Path | `D:/Dev/reverse/mole` (reverse-engineering stash, no `.git`, no commit SHA) |
| Product | Tw93 Mole 0.21.2 win-x64, .NET 10 + WinUI 3 (Windows App SDK 2.5.1), Velopack |
| Decompiled | `ilspycmd` → `src/Mole`, `Mole.Core`, `Mole.Platform`, `Mole.Presentation` |
| XAML | 17 XBF blobs carved from `Mole.pri` + string tables (`ui/xbf/`); no source XAML |
| Pixel truth | 18 runtime shots at 1216×780 + 16 UIA trees (`ui/shots/`) |
| Assets | 139 files in `ui/assets/` — **Tw93/Faberon works, not reusable** (`ui/README.md`) |
| Local project | `D:/Dev/elom` — Go 1.27, MyGo 0.8.3, MIT, no cgo |

License boundary: the stash states assets and UI are personal-RE only, "do not redistribute
or use in a product". This report ports *structure and measured tokens*, never files, glyph
paths, texture images, copy or poems.

## Source anatomy

| Layer | Where | What it shows |
|---|---|---|
| Window chrome | `MainWindow.cs`, `blob07.strings.txt` | `AppTitleBar`, `TabBar` + `TabPills` + `TabHighlight`, `TabHighlightShift`, `WashTop/Middle/Bottom`, `LogoButton`, `LogoGlow`, `StatusPillHost`, `OverlayHost` |
| Tokens | `blob08/09.strings.txt`, `Mole.App.DesignSystem/*` | `BgBaseColor`, `SurfaceCard/Control/Raised`, `Hairline`, `Fg*`, `AccentMole`, planet inks (`Earth/Jupiter/Mars/Mercury/Moon/Neptune/Saturn/Sun`), `FontSize{Footnote,Caption,Caption2,Body,Subheadline,Title,Title2,Title3,Headline,LargeTitle}`, `Radius{Xs,S,Sm,Md,Lg,Xl}`, `Spacing{S1..S8}`, `PlanetSizeHero` |
| Wash math | `TabWash.cs`, `TabWashes.cs` | per-tab colour, alpha ramp 217→107, card sink 0.15, panel lift 0.07 |
| Cards | `HudTile.cs`, `HealthCard.cs`, `MiniTile.cs` | radius 12, padding 16/14, 10 pt tracked caps title + 12×14 icon, 26 pt value + 13 pt unit, 11 pt footer, hairline `#ededee14` |
| Charts | `Sparkline.cs`, `NetworkSparkline.cs`, `PerCoreStrip.cs`, `GaugeBar.cs`, `CapacityRail.cs` | gradient fills, dual-series network, per-core bars, rounded rails |
| Planets | `PlanetStyle.cs`, `PlanetRenderer.cs` | D3D11 textured spheres, day/night terminator, per-planet channel lighting |
| Pages | `Views/*.cs` + XBF | Clean, Optimize, Software (Uninstall/Updates/Startup), Analyze, Status, Settings overlay, Details/Menu overlays |

Measured tokens (source of truth for the port):

| Token | Mole value | Evidence |
|---|---|---|
| Base background | `#15141b` | `Surface.cs:19` |
| Accent | `#4fd6a9` mint | `Surface.cs:21` |
| Tab washes | Clean `#29385c`, Apps `#47201d`, Optimize `#423d38`, Analyze `#5c3011`, Status `#574312` | `TabWashes.cs:7-15` |
| Wash alpha | 217 at top → 107 at 50% → 107 flat | `TabWash.cs:7-8,13-24` |
| Card ground | composite(base, wash), sunk 15% toward base | `TabWash.cs:33-56` |
| Card border | `rgba(237,236,238,0.078)`, 0.6 px | `HudTile.cs:65`, `Surface.cs:33` |
| Card metrics | radius 12, padding 16/14, icon 12×14 | `HudTile.cs:60-72` |
| Type | title 10 bold, letter-spacing 0.08em, uppercase | `HudTile.cs:9-13` |
| Type | value 26 bold, unit 13 semibold, footer 11 | `HudTile.cs:22-35` |
| Status grid | 4 columns; rows 164 / 164 / star | `StatusPage.cs:308-320` |
| Fonts | `Segoe UI Variable Display` / `… Text` / `Cascadia Mono, Consolas` / poem `Georgia` | `LanguageFonts.cs:20,27-29` |

## Head-to-head

| Aspect | Mole (source) | Elom today | Verdict |
|---|---|---|---|
| Window colour | base + tab wash with alpha ramp; cards are the same wash sunk 15% | two-stop gradient + 15% glow; cards are 5.5% white | Port the wash model — it is what makes every card share the tab's hue |
| Nav | logo pill left, tabs centre, status pill right, wash cross-fade | planet + tabs in a floating pill, no wash transition | Add wash cross-fade; right status pill not needed (no licence) |
| Planets | photographic textures lit in D3D | gradient sphere + blobs; reads as a blue ball with green mould | Highest visual gap. Own **ASCII character globe** with 3D rotation (decision 2026-11-10), not a copy |
| Status tiles | 4×2 fixed 164 px, icon+tracked caps, right meta, per-metric tint | 4-col auto-height grid, similar header, all charts same accent | Adopt metrics/tints; fix chart shapes |
| CPU chart | per-core bars, gradient, 12 threads | 60 tiny bars, near-invisible at 1 s | Per-core bars; sampler must expose per-core |
| Network | dual fill (down/up), labels | single series, flat when idle, rectangular fill artefact | Dual series + monotone fill; add empty-state flat baseline |
| Memory/disk | rounded gradient rails | thin flat bars | Rounded rail with gradient |
| Battery | three device rings with glyphs | one bar | Rings optional; keep percent + plug state, add charge rail |
| Process table | icon, name, mono MEM, CPU bar + %, mono PID, `…` menu | `ui.Table` text cells only | Custom row; no per-process icon (needs Exe() per PID — see challenge) |
| Apps rows | icon 28, name + publisher/version, mono size, chevron expand, hairlines | icon + name + size + checkbox, no hairlines/expand | Shared row primitive + expand leftovers |
| Segments | segmented pill in card with counts | `SegmentedBase` exists; active fill is 14% white, no counts | Align with the white pill + counts |
| Treemap | 2 px gaps, 4 px radius, top-left folder icon + name 13 + mono size 11, hover wash | 3 px gaps, radius 4, centred labels on flat fills | Label layout + gradient fill + hover ring |
| Settings | modal overlay, segmented tabs, label+description rows with control right | full page of cards | Row-based settings inside one card; overlay optional |
| Detail surfaces | `DetailsOverlay`, `MenuCard`, `CleanReviewOverlay` | inline sections | Overlay primitive (`ui.Overlay` + `Absolute`) — defer to phase 4 |
| Type | variable display/text + Cascadia + Georgia | default sans + Consolas | Adopt font stacks with Win10 fallbacks |
| Icons | 106 bespoke SVGs | 16 line icons | Extend our own line set only where a screen needs it |

## Dependency matrix

| Source component | Elom equivalent | Status |
|---|---|---|
| `Surface.cs`, `TabWash.cs`, `TabWashes.cs` | `internal/ui/design.go` `paletteFor` | CONFLICT (different model) |
| `HudTile.cs` | `internal/ui/status.go` `tile`/`tileHead` | EXISTS, metrics differ |
| `HealthCard.cs` | `status.go` `healthTile` | EXISTS |
| `Sparkline/NetworkSparkline/PerCoreStrip/CapacityRail` | `design.go` `barsChart/areaChart/lineChart/bar` | EXISTS, partial |
| `PlanetStyle/Renderer` | `design.go` `planet`/`appMark` | CONFLICT (D3D textures vs painter blobs) |
| `ProcessTable/ProcessRowView` | `status.go` `processTable` (`ui.Table`) | CONFLICT |
| `AppRowView` | `apps.go` `uninstallTab`, `software.go` leftovers | EXISTS, missing expand/icons/hairlines |
| `TreemapCanvas.cs` | `analyze.go` `paintTreemap`, `treemap.go` `squarify` | EXISTS, missing gaps/radius/labels |
| `CleanPage/CleanReviewOverlay` | `clean.go` | EXISTS, inline review |
| `SettingsOverlay` | `shell.go` `settingsPage` | CONFLICT (page vs overlay) |
| `MenuCard/DetailsOverlay` | — | NEW (optional) |
| 106 SVGs | `icons.go` 16 line icons | Recreate only what is needed |
| Poems, About, Doctor, What's New, onboarding/licence | — | OUT OF SCOPE |

## Challenges that change the plan

| # | Challenge | Source answer | Local answer | Risk if wrong |
|---|---|---|---|---|
| 1 | Can we copy visuals? | N/A | No licence to redistribute Tw93/Faberon assets or copy; MIT project | Critical: legal takedown / licence violation |
| 2 | Acrylic cards, D3D planets, blur | yes (WinUI 3) | MyGo paints gradients, bitmaps, shaders; no acrylic | Medium: fidelity ceiling; approximate |
| 3 | Is elom's current UI broken? | — | No: already Mole-shaped and functional | Medium: wasted rework if we redesign everything |
| 4 | Pixel-for-pixel clone? | — | Product differs: review-first safety, MIT, free | Medium: effort spent on non-differentiating chrome |
| 5 | Per-process icons | yes (shell icons) | Process rows carry no exe path; resolving per PID each second costs syscalls | Low: use a glyph tile instead |

Risk score: 1 critical (licence) + 1 medium (stack) → **low-medium**; proceed with
structural port, own assets, no overlay rewrite in phase 1.

## Recommendation

Port the *system*, not the pixels, in four phases:

1. Tokens + wash + card/row primitives + fonts.
2. Own ASCII character globes with 3D rotation (replaces blobs; style reference: Devault ASCII "Classic").
3. Status grid fidelity + process rows (requires a small `metrics` extension for per-core).
4. Lists and surfaces: apps rows/segments, treemap gaps/labels, settings rows, overlay primitive.

Do not port: licence/onboarding/doctor/about/poems, acrylic, D3D, third-party glyph paths,
planet texture images.

Plan: [`plans/261110-1615-elom-ui-fidelity/plan.md`](../261110-1615-elom-ui-fidelity/plan.md).
