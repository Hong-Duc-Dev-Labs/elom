package apps

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindLeftovers(t *testing.T) {
	root := t.TempDir()
	mkdir := func(rel string) string {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	local := mkdir("Local")
	roaming := mkdir("Roaming")
	mkdir("Local/Code")
	mkdir("Local/Microsoft/VS Code")
	mkdir("Roaming/Code")
	mkdir("Local/Spotify")
	mkdir("Local/Google/Chrome")

	app := App{Name: "Visual Studio Code", Publisher: "Microsoft Corporation"}
	got := FindLeftovers(app, []string{local, roaming}, nil)
	if len(got) != 3 {
		t.Fatalf("leftovers = %+v, want 3", got)
	}
	for _, l := range got {
		if filepath.Base(l.Path) == "Spotify" || filepath.Base(l.Path) == "Chrome" {
			t.Errorf("unrelated folder proposed: %s", l.Path)
		}
	}
	if got[len(got)-1].Confidence != "high" && got[len(got)-1].Confidence != "medium" {
		t.Errorf("confidence = %q", got[len(got)-1].Confidence)
	}
}

func TestFindLeftoversRespectsExclude(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "Code")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	app := App{Name: "Visual Studio Code"}
	got := FindLeftovers(app, []string{root}, func(p string) bool { return p == appDir })
	if len(got) != 0 {
		t.Fatalf("excluded folder proposed: %+v", got)
	}
}

func TestFindLeftoversSiblingGuardSkipsPublisherRoot(t *testing.T) {
	root := t.TempDir()
	pubDir := filepath.Join(root, "Adobe")
	appSub := filepath.Join(pubDir, "Photoshop")
	otherSub := filepath.Join(pubDir, "Illustrator")
	for _, d := range []string{appSub, otherSub} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	uninstalled := App{ID: "ps", Name: "Photoshop", Publisher: "Adobe Inc."}
	installed := []App{
		{ID: "ai", Name: "Illustrator", Publisher: "Adobe Inc."},
	}

	got := FindLeftoversWithSiblings(uninstalled, installed, []string{root}, nil)
	if len(got) != 1 || got[0].Path != appSub || got[0].Confidence != "high" {
		t.Fatalf("with sibling installed, leftovers = %+v, want only %s (high)", got, appSub)
	}

	// Without any sibling installed, the publisher folder itself may be proposed (medium).
	gotNoSibling := FindLeftoversWithSiblings(uninstalled, nil, []string{root}, nil)
	if len(gotNoSibling) == 0 || gotNoSibling[0].Path != pubDir {
		t.Fatalf("without sibling, leftovers = %+v, want %s", gotNoSibling, pubDir)
	}
}

func TestFindOrphanedAppliesAgeSizeAndExclusions(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	old := now.Add(-90 * 24 * time.Hour)
	recent := now.Add(-10 * 24 * time.Hour)

	makeDir := func(name string, size int, mod time.Time) string {
		t.Helper()
		d := filepath.Join(root, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		f := filepath.Join(d, "data.bin")
		if err := os.WriteFile(f, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(f, mod, mod)
		_ = os.Chtimes(d, mod, mod)
		return d
	}

	// 1. Truly orphaned, old (>60d), large (>1000 B threshold in test) -> found, Checked=false.
	orphan := makeDir("OldRemovedApp", 2048, old)
	// 2. Installed app -> must be skipped.
	makeDir("Spotify", 2048, old)
	// 3. Too recent (<60d) -> skipped.
	makeDir("RecentPortable", 2048, recent)
	// 4. Too small (<1000 B) -> skipped.
	makeDir("TinyOrphan", 100, old)
	// 5. Protected categories (AI, VPN, Password Manager, Driver) -> skipped.
	makeDir("Claude", 2048, old)
	makeDir("Tailscale", 2048, old)
	makeDir("Bitwarden", 2048, old)
	makeDir("NVIDIA", 2048, old)

	installed := []App{{Name: "Spotify", Publisher: "Spotify AB"}}
	got := FindOrphanedFiltered(installed, []string{root}, 60, 1000, now, nil)
	if len(got) != 1 {
		t.Fatalf("orphaned = %+v, want 1 item", got)
	}
	if got[0].Path != orphan || got[0].Checked || got[0].AgeDays < 60 || got[0].Bytes != 2048 {
		t.Fatalf("got[0] = %+v", got[0])
	}
}
