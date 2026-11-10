# Changelog

All notable changes to Elom are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims
at [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Documentation set: architecture, contributing guide, changelog, screenshot
  gallery and an attribution page that records Mole as the product and design
  reference ([`ATTRIBUTION.md`](ATTRIBUTION.md)).
- `StartupToggleFailed`, `cleanConfirmPermanentDetail`, `softwareUninstallFailed`,
  `softwareNoUninstaller`, localized startup source labels, process-table column
  labels and `statusKillFailed` in both message catalogs.

### Fixed

- Clean: permanent deletion now confirms with permanent wording and the
  “Delete permanently” button instead of promising a Recycle Bin restore.
- Software: a failed uninstaller is reported and no longer offers the app's
  files as leftovers, which could have removed a working program. Update and
  Uninstall stay visible but disabled while a job runs, and apps without a
  registered uninstaller explain the alternative.
- Optimize: the routine marks each task as running, shows a Cancel action and
  uses a cancellable context; single-task cancellation now uses `errors.Is`.
  Danger confirmation is marked destructive.
- Startup: toggle failures name the entry and reason instead of reusing the
  list's read error. Machine-wide entries carry the administrator tag and show
  a pending spinner while the elevated write runs.
- Status: an empty process list keeps its card with a reason, headers are
  localized, and a failed “End task” is reported instead of silently ignored.
  The health tile and chips use the shared label, number and tag components.

### Changed

- Relicensed from MIT to GPL-3.0, matching Mole and keeping the project
  compatible with adapting Mole source code. `ATTRIBUTION.md` and the README
  now state this; no Mole code is vendored yet.
- UI review outcome recorded: core design language (wash, cards, type scale,
  contrast) is shared, while several screens still hand-roll components. The
  remaining polish items are keyboard/footer hints and consolidating the muted
  text ramp.

## [0.1.0] - 2026-11-10

### Added

- Native MyGo shell with Clean, Analyze, Status, Optimize, Software, Startup and
  Settings screens; English and Vietnamese catalogs.
- Safety guard, append-only operation log, protection list and a closed
  maintenance-operation table executed through a separate elevated invocation.
- Clean rule catalog and scanner, Recycle Bin deletion with opt-in permanent
  deletion.
- Disk index, treemap and largest-files view; 1 Hz metrics sampler and process
  table; installed-app inventory, leftovers and winget updates; startup entry
  listing and toggles; maintenance tasks with per-task results.
- Per-user NSIS installer and packaged executable; bundled Geist fonts; visual
  gallery test (`ELOM_GALLERY=1`).
