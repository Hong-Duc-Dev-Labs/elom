package apps

import (
	"os"
	"path/filepath"
	"testing"
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
