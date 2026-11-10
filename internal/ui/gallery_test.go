package ui

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/apps"
	"elom/internal/core/maint"
	"elom/internal/core/scan"
	"elom/internal/core/startup"
)

// TestGallery writes one PNG per screen for a visual pass. It is a
// development aid, not a check: it is skipped unless ELOM_GALLERY=1, so the
// normal suite neither spends the render time nor writes into the repository.
func TestGallery(t *testing.T) {
	if os.Getenv("ELOM_GALLERY") == "" {
		t.Skip("set ELOM_GALLERY=1 to render the gallery")
	}
	dir := filepath.Join("..", "..", ".gallery")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	save := func(name string, a *App) {
		img := Render(a.View, 1180, 760, 1)
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}

	a := newTestApp(t)
	a.Screen = "clean"
	save("clean-empty", a)

	a = newTestApp(t)
	a.Screen = "clean"
	a.clean.scanned = true
	a.clean.candidates = []scan.Candidate{
		{Path: `C:\Users\ada\AppData\Local\Google\Chrome\User Data\Default\Cache`, Bytes: 2_400_000_000, Category: "browser", Note: "Chrome rebuilds this cache.", Checked: true},
		{Path: `C:\Users\ada\AppData\Local\Temp`, Bytes: 850_000_000, Category: "temp", Note: "Files older than a week.", Checked: true},
		{Path: `C:\Users\ada\AppData\Local\CrashDumps`, Bytes: 120_000_000, Category: "temp"},
		{Path: `C:\Users\ada\AppData\Local\Kept`, Bytes: 10, Category: "logs", Blocked: errors.New("protected by your list")},
	}
	a.clean.checked[`C:\Users\ada\AppData\Local\Google\Chrome\User Data\Default\Cache`] = true
	a.clean.checked[`C:\Users\ada\AppData\Local\Temp`] = true
	save("clean-review", a)

	a.clean.reviewing = true
	save("clean-list", a)

	a = newTestApp(t)
	a.Screen = "software"
	a.software.loaded = true
	a.software.list = []apps.App{
		{Name: "Google Chrome", Version: "131.0.6778", Publisher: "Google LLC", SizeBytes: 400_000_000},
		{Name: "Visual Studio Code", Version: "1.96.0", Publisher: "Microsoft Corporation", SizeBytes: 350_000_000},
		{Name: "Slack", Version: "4.39.90", Publisher: "Slack Technologies", SizeBytes: 250_000_000},
		{Name: "Spotify", Version: "1.2.45", Publisher: "Spotify AB", SizeBytes: 190_000_000},
	}
	a.software.picked = 0
	a.software.updatesTried = true
	save("software", a)

	a = newTestApp(t)
	a.Screen = "software"
	save("software-empty", a)

	a = newTestApp(t)
	a.Screen = "software"
	a.appsTab = 2
	a.startup.loaded = true
	a.startup.entries = []startup.Entry{
		{Name: "Discord", Command: `C:\Users\ada\AppData\Local\Discord\Update.exe --processStart`, Source: "HKCU Run", Scope: "user", Enabled: false, Writable: true},
		{Name: "Steam", Command: `C:\Program Files (x86)\Steam\steam.exe -silent`, Source: "HKLM Run", Scope: "machine", Enabled: true, Writable: true},
		{Name: "OneDrive", Command: `C:\Users\ada\AppData\Local\Microsoft\OneDrive\OneDrive.exe /background`, Source: "Startup folder", Scope: "user", Enabled: true, Writable: false, Note: "a shortcut in the Startup folder: enable or disable it there"},
	}
	save("startup", a)

	a = newTestApp(t)
	a.Screen = "optimize"
	a.optimize.results["dns.flush"] = maint.Result{Op: "dns.flush", OK: true, Detail: "Successfully flushed the DNS Resolver Cache."}
	save("optimize", a)

	a = newTestApp(t)
	a.Screen = "analyze"
	root := fakeIndex()
	a.analyze.root = root
	a.analyze.current = root
	a.analyze.largest = scan.LargestFiles(root, 20)
	save("analyze", a)

	a = newTestApp(t)
	a.Screen = "status"
	a.status = sampleSnapshot()
	save("status", a)

	a = newTestApp(t)
	a.Screen = "settings"
	a.whitelist = []string{`C:\Users\me\AppData\Local\Keep`}
	save("settings", a)
}
