package purge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSized(t *testing.T, path string, size int, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	if !mod.IsZero() {
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
}

func chtimesDir(t *testing.T, dir string, mod time.Time) {
	t.Helper()
	if err := os.Chtimes(dir, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsArtifactsAndRespectsSafetyBoundaries(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	old := now.Add(-14 * 24 * time.Hour)
	recent := now.Add(-2 * 24 * time.Hour)

	// 1. Old Node project with node_modules (and nested dist inside node_modules).
	oldApp := filepath.Join(root, "old-web")
	writeSized(t, filepath.Join(oldApp, "package.json"), 20, old)
	writeSized(t, filepath.Join(oldApp, "node_modules", "pkg", "index.js"), 4000, old)
	writeSized(t, filepath.Join(oldApp, "node_modules", "pkg", "dist", "bundle.js"), 1000, old)
	chtimesDir(t, filepath.Join(oldApp, "node_modules"), old)

	// 2. Recent Rust project -> marked Recent and unchecked by default.
	recentRust := filepath.Join(root, "fresh-rust")
	writeSized(t, filepath.Join(recentRust, "Cargo.toml"), 20, recent)
	writeSized(t, filepath.Join(recentRust, "target", "debug.bin"), 2000, recent)
	chtimesDir(t, filepath.Join(recentRust, "target"), recent)

	// 3. Random folder without project marker -> must NOT be offered.
	writeSized(t, filepath.Join(root, "notes", "build", "draft.txt"), 3000, old)

	// 4. Artifact containing a nested .git -> blocked.
	nestedGitApp := filepath.Join(root, "vendored")
	writeSized(t, filepath.Join(nestedGitApp, "package.json"), 20, old)
	writeSized(t, filepath.Join(nestedGitApp, "node_modules", "subrepo", ".git", "HEAD"), 16, old)
	writeSized(t, filepath.Join(nestedGitApp, "node_modules", "subrepo", "a.js"), 500, old)
	chtimesDir(t, filepath.Join(nestedGitApp, "node_modules"), old)

	got := Scan(context.Background(), []string{root}, Options{
		Now:          func() time.Time { return now },
		IsGitTracked: func(context.Context, string, string) bool { return false },
	}, nil)

	if len(got) != 3 {
		t.Fatalf("got %d candidates, want 3: %+v", len(got), got)
	}

	// Largest first: old-web/node_modules (5000 B), fresh-rust/target (2000 B), vendored/node_modules (500 B).
	if got[0].ProjectName != "old-web" || got[0].Artifact != "node_modules" || got[0].Bytes != 5000 {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[0].Recent || !got[0].Checked || got[0].Blocked != nil {
		t.Errorf("old-web should be checked and not recent: %+v", got[0])
	}

	if got[1].ProjectName != "fresh-rust" || !got[1].Recent || got[1].Checked {
		t.Errorf("fresh-rust should be recent and unchecked: %+v", got[1])
	}

	if got[2].ProjectName != "vendored" || !errors.Is(got[2].Blocked, ErrNestedGitRepo) || got[2].Checked {
		t.Errorf("vendored should be blocked with ErrNestedGitRepo: %+v", got[2])
	}
}

func TestScanBlocksGitTrackedArtifact(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	old := now.Add(-30 * 24 * time.Hour)

	proj := filepath.Join(root, "tracked-app")
	writeSized(t, filepath.Join(proj, ".git", "HEAD"), 20, old)
	writeSized(t, filepath.Join(proj, "package.json"), 20, old)
	writeSized(t, filepath.Join(proj, "dist", "app.js"), 1024, old)
	chtimesDir(t, filepath.Join(proj, "dist"), old)

	got := Scan(context.Background(), []string{root}, Options{
		Now: func() time.Time { return now },
		IsGitTracked: func(_ context.Context, projectDir, artifactPath string) bool {
			return filepath.Base(artifactPath) == "dist"
		},
	}, nil)

	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1", len(got))
	}
	if !errors.Is(got[0].Blocked, ErrGitTracked) || got[0].Checked {
		t.Fatalf("tracked dist should be blocked: %+v", got[0])
	}
}

func TestScanRespectsWhitelist(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	old := now.Add(-20 * 24 * time.Hour)

	proj := filepath.Join(root, "keep-me")
	writeSized(t, filepath.Join(proj, "go.mod"), 20, old)
	writeSized(t, filepath.Join(proj, "build", "out.exe"), 2048, old)
	chtimesDir(t, filepath.Join(proj, "build"), old)

	got := Scan(context.Background(), []string{root}, Options{
		Now:          func() time.Time { return now },
		Whitelist:    []string{proj},
		IsGitTracked: func(context.Context, string, string) bool { return false },
	}, nil)

	if len(got) != 1 || !errors.Is(got[0].Blocked, ErrWhitelisted) || got[0].Checked {
		t.Fatalf("whitelisted project should be blocked: %+v", got)
	}
}

func TestSuggestedPathsReturnsExistingOnly(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, "Projects")
	code := filepath.Join(home, "Code")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(code, 0o755); err != nil {
		t.Fatal(err)
	}

	got := SuggestedPaths(home)
	hasProj, hasCode := false, false
	for _, p := range got {
		if p == proj {
			hasProj = true
		}
		if p == code {
			hasCode = true
		}
	}
	if !hasProj || !hasCode {
		t.Fatalf("SuggestedPaths(%s) = %v, want %s and %s", home, got, proj, code)
	}
}
