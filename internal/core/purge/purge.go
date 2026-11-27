// Package purge finds developer build and dependency artifact directories
// (node_modules, target, .venv, bin, obj, .next, etc.) inside user-chosen
// project roots. It is pure Go with no UI dependencies and enforces strict
// safety boundaries: closed artifact names, required parent project markers,
// no nested artifact traversal, nested .git and git-tracked protection,
// 7-day recent-project unchecking, and reparse-point refusal.
package purge

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"elom/internal/core/safety"
)

const (
	// DefaultRecentDays is the window within which a modified project is
	// flagged as Recent and left unchecked by default.
	DefaultRecentDays = 7
	// DefaultMaxDepth bounds how deep project discovery descends under a root.
	DefaultMaxDepth = 6
)

var (
	// ErrNestedGitRepo is reported when an artifact directory contains a .git
	// entry (for example a vendored repository).
	ErrNestedGitRepo = errors.New("contains a nested git repository")
	// ErrGitTracked is reported when files inside the artifact are tracked by git.
	ErrGitTracked = errors.New("contains git-tracked files")
	// ErrWhitelisted is reported when the artifact matches the user's protection list.
	ErrWhitelisted = errors.New("protected by your list")
)

// Candidate is one build or dependency artifact directory found inside a project.
type Candidate struct {
	Path        string
	ProjectDir  string
	ProjectName string
	Artifact    string
	Bytes       int64
	Files       int
	Modified    time.Time
	Recent      bool
	Checked     bool
	Blocked     error
}

// Progress reports discovery progress while scanning project roots.
type Progress struct {
	ScannedDirs int
	Found       int
	Current     string
}

// Options tunes a Purge scan. Zero values use safe defaults.
type Options struct {
	Now          func() time.Time
	RecentDays   int
	MaxDepth     int
	Whitelist    []string
	IsGitTracked func(ctx context.Context, projectDir, artifactPath string) bool
}

// SuggestedPaths returns common developer workspace folders that exist on this
// machine. Callers may offer them as initial suggestions; they are never
// scanned automatically without user action.
func SuggestedPaths(home string) []string {
	if strings.TrimSpace(home) == "" {
		home = os.Getenv("USERPROFILE")
		if home == "" {
			home, _ = os.UserHomeDir()
		}
	}
	candidates := []string{
		filepath.Join(home, "Projects"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "repos"),
		filepath.Join(home, "src"),
		`D:\Dev`,
		`D:\Projects`,
	}
	var out []string
	seen := map[string]bool{}
	for _, p := range candidates {
		if strings.TrimSpace(p) == "" {
			continue
		}
		clean := filepath.Clean(p)
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		info, err := os.Stat(clean)
		if err != nil || !info.IsDir() || safety.IsReparse(clean) {
			continue
		}
		seen[key] = true
		out = append(out, clean)
	}
	return out
}

// Scan walks roots and returns discovered build/dependency artifact candidates.
func Scan(ctx context.Context, roots []string, opts Options, progress func(Progress)) []Candidate {
	if len(roots) == 0 {
		return nil
	}
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()
	recentDays := opts.RecentDays
	if recentDays <= 0 {
		recentDays = DefaultRecentDays
	}
	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	gitTrackedFn := opts.IsGitTracked
	if gitTrackedFn == nil {
		gitTrackedFn = defaultGitTracked
	}

	guard := safety.ForProjectRoots(roots...)
	recentCutoff := time.Duration(recentDays) * 24 * time.Hour

	seen := map[string]bool{}
	var out []Candidate
	var prog Progress

	emit := func(cur string) {
		if progress == nil {
			return
		}
		prog.Current = cur
		progress(prog)
	}

	var walk func(root, dir string, depth int)
	walk = func(root, dir string, depth int) {
		if ctx.Err() != nil || depth > maxDepth {
			return
		}
		if safety.IsReparse(dir) {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		prog.ScannedDirs++
		if prog.ScannedDirs%32 == 0 {
			emit(dir)
		}

		isProject := safety.HasProjectMarker(dir)
		var subdirs []string

		for _, entry := range entries {
			if ctx.Err() != nil {
				return
			}
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			name := entry.Name()
			if strings.EqualFold(name, ".git") {
				continue
			}
			full := filepath.Join(dir, name)
			if safety.IsReparse(full) {
				continue
			}

			if isProject && safety.IsArtifactName(name) {
				key := strings.ToLower(filepath.Clean(full))
				if seen[key] {
					continue
				}
				seen[key] = true

				cand := evaluateArtifact(ctx, guard, root, dir, full, name, now, recentCutoff, opts.Whitelist, gitTrackedFn)
				if cand.Bytes == 0 && cand.Files == 0 && cand.Blocked == nil {
					continue
				}
				out = append(out, cand)
				prog.Found = len(out)
				emit(full)
				// Never descend into a matched artifact directory (avoids nested artifacts).
				continue
			}

			// Even if the parent has no project marker, avoid descending into huge
			// known dependency/build trees like node_modules or .git.
			if safety.IsArtifactName(name) {
				continue
			}

			if depth+1 <= maxDepth {
				subdirs = append(subdirs, full)
			}
		}

		for _, sub := range subdirs {
			if ctx.Err() != nil {
				return
			}
			walk(root, sub, depth+1)
		}
	}

	for _, root := range roots {
		if ctx.Err() != nil {
			break
		}
		clean := filepath.Clean(strings.TrimSpace(root))
		if clean == "" {
			continue
		}
		info, err := os.Stat(clean)
		if err != nil || !info.IsDir() {
			continue
		}
		walk(clean, clean, 0)
	}
	emit("")

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		return strings.ToLower(out[i].Path) < strings.ToLower(out[j].Path)
	})
	return out
}

func evaluateArtifact(
	ctx context.Context,
	guard *safety.Guard,
	root, projectDir, artifactPath, artifactName string,
	now time.Time,
	recentCutoff time.Duration,
	whitelist []string,
	gitTrackedFn func(ctx context.Context, projectDir, artifactPath string) bool,
) Candidate {
	cand := Candidate{
		Path:        artifactPath,
		ProjectDir:  projectDir,
		ProjectName: filepath.Base(projectDir),
		Artifact:    artifactName,
	}

	size, files, hasNestedGit := measureArtifact(ctx, artifactPath)
	cand.Bytes = size
	cand.Files = files
	cand.Modified = projectModTime(projectDir, artifactPath)
	if !cand.Modified.IsZero() && now.Sub(cand.Modified) < recentCutoff {
		cand.Recent = true
	}

	switch {
	case matchesWhitelist(artifactPath, whitelist):
		cand.Blocked = ErrWhitelisted
	case guard.Check(artifactPath) != nil:
		cand.Blocked = guard.Check(artifactPath)
	case hasNestedGit:
		cand.Blocked = ErrNestedGitRepo
	case hasGitRepo(projectDir, root) && gitTrackedFn(ctx, projectDir, artifactPath):
		cand.Blocked = ErrGitTracked
	}

	cand.Checked = cand.Blocked == nil && !cand.Recent
	return cand
}

// measureArtifact sums file sizes and counts files in artifactPath without
// following symlinks or reparse points, and reports whether any nested .git
// directory or file exists inside it.
func measureArtifact(ctx context.Context, artifactPath string) (total int64, files int, nestedGit bool) {
	_ = filepath.WalkDir(artifactPath, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		if p != artifactPath && strings.EqualFold(d.Name(), ".git") {
			nestedGit = true
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || (d.IsDir() && safety.IsReparse(p)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		files++
		return nil
	})
	return total, files, nestedGit
}

// projectModTime estimates when the project was last touched by checking the
// project directory's immediate non-artifact entries and the artifact itself.
func projectModTime(projectDir, artifactPath string) time.Time {
	var latest time.Time
	consider := func(t time.Time) {
		if t.After(latest) {
			latest = t
		}
	}
	if info, err := os.Stat(artifactPath); err == nil {
		consider(info.ModTime())
	}
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return latest
	}
	for _, e := range entries {
		name := e.Name()
		if strings.EqualFold(name, ".git") || safety.IsArtifactName(name) {
			continue
		}
		if info, err := e.Info(); err == nil {
			consider(info.ModTime())
		}
	}
	return latest
}

// hasGitRepo reports whether projectDir or an ancestor up to root has a .git
// directory or file.
func hasGitRepo(projectDir, root string) bool {
	cur := filepath.Clean(projectDir)
	stop := filepath.Clean(root)
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return true
		}
		if strings.EqualFold(cur, stop) {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return false
}

// defaultGitTracked checks whether git tracks any file under artifactPath.
func defaultGitTracked(ctx context.Context, projectDir, artifactPath string) bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	rel, err := filepath.Rel(projectDir, artifactPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}
	tctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(tctx, "git", "-C", projectDir, "ls-files", "--", filepath.ToSlash(rel))
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(bytes.TrimSpace(out)) > 0
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
