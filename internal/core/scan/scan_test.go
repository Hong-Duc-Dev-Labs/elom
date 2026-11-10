package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"elom/internal/core/rules"
	"elom/internal/core/safety"
)

// fixture builds a tree under a temporary root.
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "cache", "a"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "cache", "b"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "empty"), 0o755))
	must(os.WriteFile(filepath.Join(root, "cache", "a", "one.bin"), make([]byte, 100), 0o644))
	must(os.WriteFile(filepath.Join(root, "cache", "a", "two.bin"), make([]byte, 50), 0o644))
	must(os.WriteFile(filepath.Join(root, "cache", "b", "three.bin"), make([]byte, 25), 0o644))
	must(os.WriteFile(filepath.Join(root, "single.bin"), make([]byte, 10), 0o644))
	old := time.Now().Add(-30 * 24 * time.Hour)
	must(os.Chtimes(filepath.Join(root, "cache", "b", "three.bin"), old, old))
	return root
}

func TestRunSizesAndSorts(t *testing.T) {
	root := fixture(t)
	guard := safety.ForRoots(root)
	catalog := []rules.Rule{
		{ID: "cache", Category: "app-cache", App: "Test", Paths: []string{filepath.Join(root, "cache", "*")}, DefaultChecked: true, Regenerates: true},
		{ID: "single", Category: "logs", App: "Test", Paths: []string{filepath.Join(root, "single.bin")}},
	}
	var progress int
	got := Run(context.Background(), guard, catalog, Options{}, func(Progress) { progress++ })

	if len(got) != 3 {
		t.Fatalf("got %d candidates, want 3: %+v", len(got), got)
	}
	if progress == 0 {
		t.Error("no progress reported")
	}
	if got[0].Path != filepath.Join(root, "cache", "a") || got[0].Bytes != 150 || got[0].Files != 2 {
		t.Errorf("largest = %+v, want cache/a with 150 bytes in 2 files", got[0])
	}
	if got[1].Bytes != 25 || got[2].Bytes != 10 {
		t.Errorf("order = %d, %d, want 25 then 10", got[1].Bytes, got[2].Bytes)
	}
	if !got[0].Checked || !got[0].Regenerates {
		t.Errorf("candidate flags = checked %v regenerates %v", got[0].Checked, got[0].Regenerates)
	}
}

func TestRunHonoursWhitelist(t *testing.T) {
	root := fixture(t)
	guard := safety.ForRoots(root)
	catalog := []rules.Rule{{ID: "cache", Category: "app-cache", Paths: []string{filepath.Join(root, "cache", "*")}}}
	got := Run(context.Background(), guard, catalog, Options{Whitelist: []string{filepath.Join(root, "cache", "a")}}, nil)
	for _, c := range got {
		if filepath.Base(c.Path) == "a" && c.Blocked == nil {
			t.Errorf("whitelisted path %s was not blocked", c.Path)
		}
		if filepath.Base(c.Path) == "b" && c.Blocked != nil {
			t.Errorf("unprotected path %s was blocked: %v", c.Path, c.Blocked)
		}
	}
}

func TestRunAgeFilter(t *testing.T) {
	root := fixture(t)
	guard := safety.ForRoots(root)
	catalog := []rules.Rule{{
		ID: "old", Category: "system-temp",
		Paths:      []string{filepath.Join(root, "cache", "b", "*")},
		MinAgeDays: 7,
	}}
	got := Run(context.Background(), guard, catalog, Options{}, nil)
	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1", len(got))
	}
	if got[0].Blocked != nil {
		t.Fatalf("old file blocked: %v", got[0].Blocked)
	}
	catalog[0].MinAgeDays = 60
	got = Run(context.Background(), guard, catalog, Options{}, nil)
	if len(got) != 0 {
		t.Fatalf("a 30-day-old file passed a 60-day rule: %+v", got)
	}
}

func TestRunRefusesOutsideRoot(t *testing.T) {
	root := fixture(t)
	guard := safety.ForRoots(root)
	other := t.TempDir()
	catalog := []rules.Rule{{ID: "escape", Category: "logs", Paths: []string{filepath.Join(other, "x")}}}
	if err := os.WriteFile(filepath.Join(other, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Run(context.Background(), guard, catalog, Options{}, nil)
	if len(got) != 1 || got[0].Blocked == nil {
		t.Fatalf("path outside the allowed root was not blocked: %+v", got)
	}
}

func TestRunCancel(t *testing.T) {
	root := fixture(t)
	guard := safety.ForRoots(root)
	catalog := []rules.Rule{{ID: "cache", Category: "app-cache", Paths: []string{filepath.Join(root, "cache", "*")}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := Run(ctx, guard, catalog, Options{}, nil)
	for _, c := range got {
		if c.Blocked == nil {
			t.Errorf("candidate %s measured after cancel", c.Path)
		}
	}
	_ = got
}
