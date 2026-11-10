package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/apps"
	"elom/internal/core/startup"
	"elom/internal/core/winget"
)

func TestSoftwarePageRendersApps(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.software.loaded = true
	a.software.list = []apps.App{
		{Name: "Google Chrome", Version: "131.0", Publisher: "Google LLC", SizeBytes: 400_000_000},
		{Name: "Slack", Version: "4.39", Publisher: "Slack Technologies", SizeBytes: 250_000_000},
	}
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"Google Chrome", "Slack", "131.0", "2 apps"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
}

func TestSoftwareSearchFilters(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.software.loaded = true
	a.software.list = []apps.App{
		{Name: "Google Chrome", Publisher: "Google LLC"},
		{Name: "Slack", Publisher: "Slack Technologies"},
	}
	a.software.query = "slack"
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "Slack") || strings.Contains(texts, "Google Chrome") {
		t.Errorf("filter failed:\n%s", texts)
	}
	if !strings.Contains(texts, "1 apps") {
		t.Errorf("count not filtered:\n%s", texts)
	}
}

func TestSoftwareEmptyAndError(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.software.err = "powershell is missing"
	tt := NewTester(a.View, 1180, 760)
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "Could not read installed apps") {
		t.Errorf("error not shown: %q", tt.Texts())
	}
}

func TestStartupPageRendersEntries(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.appsTab = 2
	a.startup.loaded = true
	a.startup.entries = []startup.Entry{
		{Name: "Discord", Command: `C:\Discord\Update.exe --processStart`, Source: "HKCU Run", Scope: "user", Enabled: false, Writable: true},
		{Name: "Steam.lnk", Command: `C:\ProgramData\...\Steam.lnk`, Source: "Startup folder (all users)", Scope: "machine", Enabled: true, Writable: false, Note: "a shortcut in the Startup folder: enable or disable it there"},
	}
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"Discord", "HKCU Run", "Turned off", "Steam.lnk", "Startup folder"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
}

func TestStartupEmpty(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.appsTab = 2
	a.startup.loaded = true
	tt := NewTester(a.View, 1180, 760)
	if !tt.HasText("Nothing starts with Windows.") {
		t.Errorf("empty state missing: %q", tt.Texts())
	}
}

func TestSoftwareUpdatesSection(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.appsTab = 1
	a.software.loaded = true
	a.software.updatesTried = true
	a.software.updates = []winget.Upgrade{
		{Name: "Git", ID: "Git.Git", Current: "2.45.1", Available: "2.46.0"},
	}
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"Updates", "Git", "Git.Git", "2.45.1 → 2.46.0"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
}

func TestSoftwareUpdatesEmpty(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.appsTab = 1
	a.software.loaded = true
	a.software.updatesTried = true
	tt := NewTester(a.View, 1180, 760)
	if !tt.HasText("Everything winget manages is up to date.") {
		t.Errorf("empty updates state missing: %q", tt.Texts())
	}
}

func TestSoftwareUpdatesUnparseable(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.appsTab = 1
	a.software.loaded = true
	a.software.updatesErr = "winget: unrecognized output"
	tt := NewTester(a.View, 1180, 760)
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "could not be read") {
		t.Errorf("unparseable state missing: %q", tt.Texts())
	}
}

func TestLeftoversRenderMeasuredBytes(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.software.loaded = true
	a.software.leftovers = []apps.Leftover{
		{Path: `C:\Users\ada\AppData\Local\Code`, Confidence: "high", Reason: "name matches the app", Bytes: 2048},
	}
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "2.0 kB") {
		t.Errorf("leftover size not drawn from the measured value:\n%s", texts)
	}
}

func TestSoftwareWarningsShown(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.software.loaded = true
	a.software.warnings = []string{"apps: Get-AppxPackage: powershell.exe not found"}
	tt := NewTester(a.View, 1180, 760)
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "Get-AppxPackage") {
		t.Errorf("warning not shown: %q", tt.Texts())
	}
}

func TestIconPath(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "app.exe")
	if err := os.WriteFile(exe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := iconPath(apps.App{Icon: exe + ",0"}); got != exe {
		t.Errorf("iconPath = %q, want %q", got, exe)
	}
	if got := iconPath(apps.App{Icon: filepath.Join(dir, "missing.exe")}); got != "" {
		t.Errorf("iconPath(missing) = %q", got)
	}
	if got := iconPath(apps.App{}); got != "" {
		t.Errorf("iconPath(empty) = %q", got)
	}
}

func TestSoftwareDetailRendersSelectedApp(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.software.loaded = true
	a.software.list = []apps.App{{
		Name: "Google Chrome", Version: "131.0", Publisher: "Google LLC",
		InstallLocation: `C:\Program Files\Google\Chrome\Application`,
		Uninstall:       `C:\chrome\setup.exe --uninstall`,
	}}
	a.software.picked = 0
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"Google LLC", `C:\Program Files\Google\Chrome\Application`, "Show in Explorer"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
}
