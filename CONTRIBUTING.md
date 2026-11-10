# Contributing

Elom is a Go native Windows utility built on
[MyGo](https://mygo.egoist.dev). This guide covers the checks a change is
expected to pass. Read [`docs/architecture.md`](docs/architecture.md) before
touching the UI, and [`SECURITY.md`](SECURITY.md) before touching deletion or
elevation.

## Requirements

- Go 1.27 or later (the MyGo CLI is pinned as a Go tool in `go.mod`).
- Windows 10/11 for the real-machine probes and for running the app.
- No cgo, no Node toolchain, no webview.

## Build and test

```powershell
go tool mygo dev                       # development build with live reload
go tool mygo build                     # build/windows-amd64/Elom.exe + NSIS installer
go vet ./...
go test ./... -count=1
go tool mygo i18n check .
```

Before a commit, run the full suite. During development, run the narrowest test
that covers the change, for example:

```powershell
go test ./internal/ui -run TestRoutineMarksTheRunningTask -count=1
```

Opt-in probes read the real machine and are skipped by default; they are listed
in [`docs/verification.md`](docs/verification.md).

## Visual checks

The UI is rendered headlessly into PNGs for review:

```powershell
$env:ELOM_GALLERY=1; go test ./internal/ui -run TestGallery -count=1
```

The images land in `.gallery/` (gitignored). Treat them as development
evidence; when a screenshot is committed under `docs/screenshots/`, regenerate
it from the current code.

## Messages and localization

English is the source catalog. To add or change a string:

1. edit [`locales/en.json`](locales/en.json) and [`locales/vi.json`](locales/vi.json);
2. run `go tool mygo generate` to regenerate `locales/catalog_gen.go`;
3. run `go tool mygo i18n check .` and fix any missing or mismatched arguments;
4. render the affected screen and check that Vietnamese diacritics and longer
   strings still fit.

Do not edit `locales/catalog_gen.go` by hand.

## Design system

The native UI follows Mole's measured design language, credited in
[`ATTRIBUTION.md`](ATTRIBUTION.md). Use the shared pieces in
`internal/ui/design.go` instead of inventing local ones:

- `card`, `cardWith`, `cardHead`, `rule` for panels;
- `capsLabel` for tracked small capitals;
- `listRow`, `rowDivider`, `pillTag`, `toggle`, `bar`;
- `bigNumber`, `hintText`, `primaryPill`, `ghostPill`;
- `fgPrimary` / `fgSecondary` / `fgMuted` / `fgFaint` for text roles;
- `paletteFor(id)` for a screen's wash, glow and ink.

A screen should handle its empty, loading, error and destructive-confirmation
states with the same components and wording as its neighbours.

## Boundaries

- `internal/core` must not import MyGo (enforced by
  `internal/core/arch_test.go`); it is pure Go and testable off-Windows where
  possible.
- `internal/svc` may use MyGo for shell integration; `internal/ui` owns the
  native widgets and the UI-thread state.
- The deletion guard is checked twice: during the scan and again at the moment
  of deletion. Do not bypass it.
- The UI never runs elevated. New privileged work must be added to the closed
  operation table in `internal/core/maint` and re-validated by the elevated
  process.

## Commits

Use conventional commits (`fix(ui): …`, `feat(core): …`, `docs: …`,
`test: …`). Keep the message about behaviour, not the file list. Do not commit
`build/`, `.mygo/`, `.gallery/`, `coverage.out` or generated executables.
