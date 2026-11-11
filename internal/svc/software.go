package svc

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"elom/internal/core/apps"
	"elom/internal/core/scan"
	"elom/internal/core/winget"
)

// Software lists apps, runs their uninstallers and finds what they leave.
type Software struct {
	// Home, Local, Roaming and ProgramData are the leftover roots; zero
	// values skip that root.
	Home, Local, Roaming, ProgramData string
	// Exclude keeps a leftover from being proposed, usually the safety
	// guard.
	Exclude func(string) bool
	// EnumerateRegistry and EnumeratePackaged default to the registry and
	// PowerShell; tests replace them.
	EnumerateRegistry func() ([]apps.App, error)
	EnumeratePackaged func(context.Context) ([]apps.App, error)
	// Measure sizes a leftover; tests replace it.
	Measure func(string) int64
}

// List returns registry apps, and packaged apps when PowerShell answers.
// A packaged-app failure is not fatal, but it is reported as a warning so the
// list does not silently look complete.
func (s *Software) List(ctx context.Context) ([]apps.App, []string, error) {
	registry := s.EnumerateRegistry
	if registry == nil {
		registry = apps.Enumerate
	}
	packagedFn := s.EnumeratePackaged
	if packagedFn == nil {
		packagedFn = apps.EnumeratePackaged
	}
	list, err := registry()
	if err != nil {
		return nil, nil, err
	}
	var warnings []string
	if packaged, perr := packagedFn(ctx); perr != nil {
		warnings = append(warnings, perr.Error())
	} else {
		list = append(list, packaged...)
	}
	apps.SortApps(list)
	return list, warnings, nil
}

// Uninstall runs the app's own uninstaller, streaming its output.
func (s *Software) Uninstall(ctx context.Context, app apps.App, onLine func(string)) error {
	return apps.RunUninstall(ctx, app.UninstallCommand(), onLine)
}

// StillInstalled reports whether the app's uninstall entry is still in the
// registry.
func (s *Software) StillInstalled(app apps.App) (bool, error) {
	registry := s.EnumerateRegistry
	if registry == nil {
		registry = apps.Enumerate
	}
	list, err := registry()
	if err != nil {
		return false, err
	}
	for _, other := range list {
		if other.ID != app.ID {
			continue
		}
		// Some uninstallers remove the program and leave their registry key
		// behind. When the key names an install folder and that folder is
		// gone, the app is gone too.
		if app.InstallLocation != "" {
			if _, err := os.Stat(app.InstallLocation); errors.Is(err, fs.ErrNotExist) {
				return false, nil
			}
		}
		return true, nil
	}
	return false, nil
}

// AwaitRemoval waits until the app's uninstall entry is gone, checking every
// interval for at most grace. The uninstaller exiting proves nothing: many
// copy themselves to %TEMP%, relaunch and exit at once, and the copy may
// still be running or may have been cancelled. It reports whether the app is
// gone; an unreadable registry counts as not gone.
func (s *Software) AwaitRemoval(ctx context.Context, app apps.App, grace, interval time.Duration) bool {
	deadline := time.Now().Add(grace)
	for {
		installed, err := s.StillInstalled(app)
		if err == nil && !installed {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(interval):
		}
	}
}

// Leftovers returns the folders that probably belong to the app, with their
// sizes measured here, once. It needs the app's uninstall to have finished, or
// the vendor uninstaller would race it.
func (s *Software) Leftovers(app apps.App) []apps.Leftover {
	roots := apps.LeftoverRoots(s.Home, s.Local, s.Roaming, s.ProgramData)
	leftovers := apps.FindLeftovers(app, roots, s.Exclude)
	measure := s.Measure
	if measure == nil {
		measure = func(path string) int64 { bytes, _ := scan.Size(path); return bytes }
	}
	for i := range leftovers {
		leftovers[i].Bytes = measure(leftovers[i].Path)
	}
	return leftovers
}

// Updates asks winget which installed packages have newer versions.
func (s *Software) Updates(ctx context.Context) ([]winget.Upgrade, error) {
	return winget.List(ctx)
}

// ApplyUpdate updates one package by id, streaming winget's output.
func (s *Software) ApplyUpdate(ctx context.Context, id string, onLine func(string)) error {
	return winget.Apply(ctx, id, onLine)
}

// RootsFromUser fills the leftover roots from the current user's environment.
func RootsFromUser() (home, local, roaming, programData string) {
	home = userHome()
	local = filepath.Join(home, "AppData", "Local")
	roaming = filepath.Join(home, "AppData", "Roaming")
	programData = `C:\ProgramData`
	return home, local, roaming, programData
}
