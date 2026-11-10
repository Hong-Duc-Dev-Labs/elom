package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/oplog"
	"elom/internal/core/settings"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	store, err := settings.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	log, err := oplog.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return New(dir, store, log)
}

func TestShellOpensOnClean(t *testing.T) {
	a := newTestApp(t)
	tt := NewTester(a.View, 1180, 760)
	if a.Screen != "clean" {
		t.Fatalf("screen = %q, want clean", a.Screen)
	}
	for _, label := range []string{"Clean", "Apps", "Optimize", "Analyze", "Status"} {
		if !tt.HasText(label) {
			t.Errorf("tabs are missing %q; texts: %q", label, tt.Texts())
		}
	}
	if err := tt.Click("Settings"); err != nil {
		t.Fatalf("no Settings button: %v", err)
	}
	if a.Screen != "settings" {
		t.Errorf("screen = %q after clicking Settings, want settings", a.Screen)
	}
}

func TestSidebarNavigates(t *testing.T) {
	a := newTestApp(t)
	tt := NewTester(a.View, 1180, 760)
	if err := tt.Click("Analyze"); err != nil {
		t.Fatal(err)
	}
	if a.Screen != "analyze" {
		t.Fatalf("screen = %q after clicking Analyze, want analyze", a.Screen)
	}
	if !tt.HasText("Choose a folder to map what fills the disk.") {
		t.Errorf("analyze page not drawn; texts: %q", tt.Texts())
	}
}

func TestSettingsShowsGuardAndLog(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "settings"
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "operations.log") {
		t.Errorf("settings does not show the log path:\n%s", texts)
	}
	if !strings.Contains(texts, "writable") {
		t.Errorf("settings does not show the log state:\n%s", texts)
	}
	roots := a.Guard.AllowRoots()
	if len(roots) == 0 {
		t.Fatal("no allow roots")
	}
	if !strings.Contains(texts, roots[0]) {
		t.Errorf("settings does not list %q:\n%s", roots[0], texts)
	}
}

func TestSettingsReportsCorruptWhitelist(t *testing.T) {
	a := newTestApp(t)
	path := filepath.Join(a.DataDir, "whitelist.json")
	if err := os.WriteFile(path, []byte("[oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.reload()
	a.Screen = "settings"
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "unreadable") {
		t.Errorf("settings does not report the unreadable whitelist:\n%s", texts)
	}
}
