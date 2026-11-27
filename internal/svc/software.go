package svc

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"elom/internal/core/apps"
	"elom/internal/core/scan"
	"elom/internal/core/winget"
)

// DefaultSoftwareCacheTTL is how long Software.List caches enumerated apps
// before refreshing automatically. Mutations (Uninstall, ApplyUpdate) always
// invalidate the cache immediately.
const DefaultSoftwareCacheTTL = 5 * time.Minute

// UpdateAllResult records the outcome of updating one package in UpdateAll.
type UpdateAllResult struct {
	ID   string
	Name string
	Err  string
}

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
	// ApplyUpdateFn overrides winget.Apply; tests replace it.
	ApplyUpdateFn func(context.Context, string, func(string)) error
	// Measure sizes a leftover; tests replace it.
	Measure func(string) int64
	// CacheTTL overrides DefaultSoftwareCacheTTL when > 0.
	CacheTTL time.Duration

	cacheMu        sync.Mutex
	cachedList     []apps.App
	cachedWarnings []string
	cachedAt       time.Time
}

// InvalidateCache clears the cached app list so the next List call re-scans.
func (s *Software) InvalidateCache() {
	s.cacheMu.Lock()
	s.cachedList = nil
	s.cachedWarnings = nil
	s.cachedAt = time.Time{}
	s.cacheMu.Unlock()
}

// Refresh invalidates the cache and performs a fresh enumeration.
func (s *Software) Refresh(ctx context.Context) ([]apps.App, []string, error) {
	s.InvalidateCache()
	return s.List(ctx)
}

// List returns registry apps, and packaged apps when PowerShell answers.
// Results are cached in memory until CacheTTL expires or InvalidateCache is
// called (for example after Uninstall, ApplyUpdate, or user Refresh).
// A packaged-app failure is not fatal, but it is reported as a warning so the
// list does not silently look complete.
func (s *Software) List(ctx context.Context) ([]apps.App, []string, error) {
	ttl := s.CacheTTL
	if ttl <= 0 {
		ttl = DefaultSoftwareCacheTTL
	}
	s.cacheMu.Lock()
	if !s.cachedAt.IsZero() && time.Since(s.cachedAt) < ttl {
		list := append([]apps.App(nil), s.cachedList...)
		warnings := append([]string(nil), s.cachedWarnings...)
		s.cacheMu.Unlock()
		return list, warnings, nil
	}
	s.cacheMu.Unlock()

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

	s.cacheMu.Lock()
	s.cachedList = append([]apps.App(nil), list...)
	s.cachedWarnings = append([]string(nil), warnings...)
	s.cachedAt = time.Now()
	s.cacheMu.Unlock()

	return list, warnings, nil
}

// Uninstall runs the app's own uninstaller, streaming its output, and
// invalidates the app scan cache.
func (s *Software) Uninstall(ctx context.Context, app apps.App, onLine func(string)) error {
	defer s.InvalidateCache()
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
// sizes measured here, once. It applies the sibling-install guard against
// currently installed apps so shared publisher folders are never proposed when
// another app from the same publisher remains installed.
func (s *Software) Leftovers(app apps.App) []apps.Leftover {
	registry := s.EnumerateRegistry
	if registry == nil {
		registry = apps.Enumerate
	}
	installed, _ := registry()
	return s.LeftoversWithInstalled(app, installed)
}

// LeftoversWithInstalled runs leftover discovery with an explicit list of
// currently installed apps for the sibling-install guard.
func (s *Software) LeftoversWithInstalled(app apps.App, installed []apps.App) []apps.Leftover {
	roots := apps.LeftoverRoots(s.Home, s.Local, s.Roaming, s.ProgramData)
	leftovers := apps.FindLeftoversWithSiblings(app, installed, roots, s.Exclude)
	measure := s.Measure
	if measure == nil {
		measure = func(path string) int64 { bytes, _ := scan.Size(path); return bytes }
	}
	for i := range leftovers {
		leftovers[i].Bytes = measure(leftovers[i].Path)
	}
	return leftovers
}

// FindOrphaned scans user AppData folders (Local and Roaming) for inactive
// (>60 days), large (>10 MB) directories that do not match any installed app
// and are not protected system/driver/framework/AI/VPN/password-manager data.
func (s *Software) FindOrphaned(ctx context.Context) ([]apps.Leftover, error) {
	installed, _, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	var roots []string
	if s.Local != "" {
		roots = append(roots, s.Local)
	}
	if s.Roaming != "" {
		roots = append(roots, s.Roaming)
	}
	return apps.FindOrphanedFiltered(
		installed,
		roots,
		apps.DefaultOrphanMinAgeDays,
		apps.DefaultOrphanMinBytes,
		time.Now(),
		s.Exclude,
	), nil
}

// Updates asks winget which installed packages have newer versions.
func (s *Software) Updates(ctx context.Context) ([]winget.Upgrade, error) {
	return winget.List(ctx)
}

// ApplyUpdate updates one package by id, streaming winget's output, and
// invalidates the app scan cache on completion.
func (s *Software) ApplyUpdate(ctx context.Context, id string, onLine func(string)) error {
	fn := s.ApplyUpdateFn
	if fn == nil {
		fn = winget.Apply
	}
	err := fn(ctx, id, onLine)
	if err == nil {
		s.InvalidateCache()
	}
	return err
}

// UpdateAll updates packages sequentially, stopping early if ctx is cancelled.
// Each completed package is recorded in the returned slice.
func (s *Software) UpdateAll(ctx context.Context, updates []winget.Upgrade, onProgress func(index, total int, u winget.Upgrade), onLine func(string)) []UpdateAllResult {
	results := make([]UpdateAllResult, 0, len(updates))
	for i, u := range updates {
		if ctx.Err() != nil {
			break
		}
		if onProgress != nil {
			onProgress(i+1, len(updates), u)
		}
		err := s.ApplyUpdate(ctx, u.ID, onLine)
		res := UpdateAllResult{ID: u.ID, Name: u.Name}
		if err != nil {
			res.Err = err.Error()
		}
		results = append(results, res)
	}
	return results
}

// RootsFromUser fills the leftover roots from the current user's environment.
func RootsFromUser() (home, local, roaming, programData string) {
	home = userHome()
	local = filepath.Join(home, "AppData", "Local")
	roaming = filepath.Join(home, "AppData", "Roaming")
	programData = `C:\ProgramData`
	return home, local, roaming, programData
}
