# Attribution and prior art

## Mole

Elom is an independent Windows implementation of the experience of
**[Mole](https://github.com/tw93/Mole)**, the open-source cleaner by tw93 and
contributors. Mole is the product and design reference for this project.

- **Mole repository:** <https://github.com/tw93/Mole>
- **Mole license:** GPL-3.0

### What Elom takes from Mole

- the product concept and screen structure: Clean, Analyze, Status, Optimize,
  Software, Startup and Settings;
- the visual design measured from Mole's native UI: the per-screen tab wash
  (alpha 217 → 107), the card ground cut from that wash, the type scale and
  tracked small capitals, the character-grid globes and the status tiles;
- the safety posture: review before delete, a protection list, and privileged
  work in a separate validated invocation.

These choices are recorded in [`internal/ui/design.go`](internal/ui/design.go)
and [`docs/architecture.md`](docs/architecture.md).

### What Elom does not include

Elom contains **no Mole source code, assets, glyph paths, icons or logo**, and
this repository is not a fork of Mole. The implementation is Go on
[MyGo](https://mygo.egoist.dev); Mole's own code is not vendored, linked or
executed.

### Licensing note

Elom is released under the MIT license ([`LICENSE`](LICENSE)); Mole is
GPL-3.0. An independent reimplementation does not import GPL-licensed code, so
Elom can remain MIT. If you copy or adapt Mole source code into this project,
the combined work must be distributed under GPL-3.0 and this file must be
updated. Do not copy Mole assets or icons without checking their license.

"Mole" is used descriptively to credit the prior art; the name and logo belong
to the Mole authors.

## Bundled third-party software

| Component | License | Where |
|---|---|---|
| [MyGo](https://github.com/egoist/mygo) | MIT | UI framework and build tool |
| [Geist and Geist Mono](https://vercel.com/font) | SIL Open Font License 1.1 | `internal/ui/fonts/` (see `OFL.txt`) |
| Go modules in `go.mod` | their own licenses | linked dependencies (gopsutil, golang.org/x, …) |

The bundled Geist families are redistributed under the SIL OFL; the license
text ships with the fonts in [`internal/ui/fonts/OFL.txt`](internal/ui/fonts/OFL.txt).
