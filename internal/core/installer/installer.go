// Package installer finds old Windows installer files (.msi, .msix, .iso,
// .exe) in the user's Downloads folder (levels 1-2). It is pure Go with no UI
// dependencies, skips symlinks/reparse points, leaves .exe files unchecked by
// default to protect portable tools, and pairs with safety.ForInstallerDownloads
// so deletion always targets single files and always goes to the Recycle Bin.
package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"elom/internal/core/safety"
)

const (
	// DefaultMinAgeDays is the minimum file age before an installer is offered.
	DefaultMinAgeDays = 14
)

// ErrWhitelisted is reported when an installer matches the user's protection list.
var ErrWhitelisted = errors.New("protected by your list")

// Candidate is one old installer file found in Downloads.
type Candidate struct {
	Path     string
	Name     string
	Ext      string
	Bytes    int64
	Modified time.Time
	AgeDays  int
	Checked  bool
	Blocked  error
}

// Options tunes an installer scan. Zero values use safe defaults.
type Options struct {
	MinAgeDays int
	Now        func() time.Time
	Whitelist  []string
}

// DefaultDownloadsDir returns %USERPROFILE%\Downloads (or home/Downloads).
func DefaultDownloadsDir(home string) string {
	if strings.TrimSpace(home) == "" {
		home = os.Getenv("USERPROFILE")
		if home == "" {
			home, _ = os.UserHomeDir()
		}
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, "Downloads")
}

// Scan inspects levels 1 and 2 of downloadsDir for old installer files.
// Symlinks and reparse points are never followed or offered.
func Scan(ctx context.Context, downloadsDir string, opts Options) []Candidate {
	cleanDir := filepath.Clean(strings.TrimSpace(downloadsDir))
	if cleanDir == "" || cleanDir == "." {
		return nil
	}
	if safety.IsReparse(cleanDir) {
		return nil
	}
	info, err := os.Stat(cleanDir)
	if err != nil || !info.IsDir() {
		return nil
	}

	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()
	minAgeDays := opts.MinAgeDays
	if minAgeDays <= 0 {
		minAgeDays = DefaultMinAgeDays
	}
	minAge := time.Duration(minAgeDays) * 24 * time.Hour

	guard := safety.ForInstallerDownloads(cleanDir)
	var out []Candidate

	scanDir := func(dir string, depth int) []string {
		if ctx.Err() != nil {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		var subdirs []string
		for _, entry := range entries {
			if ctx.Err() != nil {
				return nil
			}
			full := filepath.Join(dir, entry.Name())
			if entry.Type()&os.ModeSymlink != 0 || safety.IsReparse(full) {
				continue
			}
			if entry.IsDir() {
				if depth == 1 {
					subdirs = append(subdirs, full)
				}
				continue
			}
			if !safety.IsInstallerExt(entry.Name()) {
				continue
			}
			finfo, err := entry.Info()
			if err != nil || !finfo.Mode().IsRegular() {
				continue
			}
			age := now.Sub(finfo.ModTime())
			if age < minAge {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			cand := Candidate{
				Path:     full,
				Name:     entry.Name(),
				Ext:      ext,
				Bytes:    finfo.Size(),
				Modified: finfo.ModTime(),
				AgeDays:  int(age / (24 * time.Hour)),
			}
			switch {
			case matchesWhitelist(full, opts.Whitelist):
				cand.Blocked = ErrWhitelisted
			case guard.Check(full) != nil:
				cand.Blocked = guard.Check(full)
			}
			// Leave .exe unchecked by default to avoid removing portable tools.
			cand.Checked = cand.Blocked == nil && ext != ".exe"
			out = append(out, cand)
		}
		return subdirs
	}

	// Level 1: directly in Downloads.
	subdirs := scanDir(cleanDir, 1)
	// Level 2: one folder deep inside Downloads.
	for _, sub := range subdirs {
		if ctx.Err() != nil {
			break
		}
		scanDir(sub, 2)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		return strings.ToLower(out[i].Path) < strings.ToLower(out[j].Path)
	})
	return out
}

func matchesWhitelist(path string, patterns []string) bool {
	norm := strings.ToLower(filepath.Clean(path))
	for _, raw := range patterns {
		pat := strings.ToLower(filepath.Clean(strings.TrimSpace(raw)))
		if pat == "" {
			continue
		}
		if norm == pat || strings.HasPrefix(norm, pat+string(os.PathSeparator)) {
			return true
		}
		if ok, err := filepath.Match(pat, norm); err == nil && ok {
			return true
		}
	}
	return false
}
