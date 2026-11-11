package svc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"elom/internal/core/apps"
)

func TestListReportsPackagedFailure(t *testing.T) {
	sw := &Software{
		EnumerateRegistry: func() ([]apps.App, error) {
			return []apps.App{{Name: "Registry App", Kind: apps.KindRegistry}}, nil
		},
		EnumeratePackaged: func(context.Context) ([]apps.App, error) {
			return nil, errors.New("powershell.exe not found")
		},
	}
	list, warnings, err := sw.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Registry App" {
		t.Fatalf("list = %+v", list)
	}
	if len(warnings) != 1 || warnings[0] == "" {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestListFailsWhenRegistryFails(t *testing.T) {
	sw := &Software{
		EnumerateRegistry: func() ([]apps.App, error) { return nil, errors.New("no registry") },
	}
	if _, _, err := sw.List(context.Background()); err == nil {
		t.Fatal("List hid a registry failure")
	}
}

func TestAwaitRemovalWaitsForTheEntryToGo(t *testing.T) {
	app := apps.App{ID: `HKCU 64\Vendor App`, Name: "Vendor App", Kind: apps.KindRegistry}
	calls := 0
	sw := &Software{EnumerateRegistry: func() ([]apps.App, error) {
		calls++
		if calls < 3 {
			return []apps.App{app}, nil
		}
		return nil, nil
	}}
	if !sw.AwaitRemoval(context.Background(), app, time.Second, time.Millisecond) {
		t.Fatal("the entry disappeared, but AwaitRemoval said the app is still installed")
	}
	if calls != 3 {
		t.Errorf("registry read %d times, want 3", calls)
	}
}

// An uninstaller that removed the program but left its registry key behind
// must not hold the screen for the whole grace period.
func TestStillInstalledIgnoresAKeyWhoseInstallFolderIsGone(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "removed")
	app := apps.App{ID: `HKCU 64\Vendor App`, Name: "Vendor App", InstallLocation: gone}
	sw := &Software{EnumerateRegistry: func() ([]apps.App, error) { return []apps.App{app}, nil }}
	installed, err := sw.StillInstalled(app)
	if err != nil || installed {
		t.Fatalf("StillInstalled = %v, %v; want false with the folder gone", installed, err)
	}

	app.InstallLocation = t.TempDir()
	installed, err = sw.StillInstalled(app)
	if err != nil || !installed {
		t.Fatalf("StillInstalled = %v, %v; want true while the folder exists", installed, err)
	}

	app.InstallLocation = ""
	installed, err = sw.StillInstalled(app)
	if err != nil || !installed {
		t.Fatalf("StillInstalled = %v, %v; want true when no folder is recorded", installed, err)
	}
}

func TestAwaitRemovalStopsWhenCancelled(t *testing.T) {
	app := apps.App{ID: `HKCU 64\Vendor App`, Name: "Vendor App"}
	sw := &Software{EnumerateRegistry: func() ([]apps.App, error) { return []apps.App{app}, nil }}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	start := time.Now()
	if sw.AwaitRemoval(ctx, app, time.Minute, 5*time.Millisecond) {
		t.Fatal("a cancelled wait reported the app as removed")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("cancelling did not end the wait")
	}
}

func TestAwaitRemovalGivesUpWhileStillInstalled(t *testing.T) {
	app := apps.App{ID: `HKCU 64\Vendor App`, Name: "Vendor App", Kind: apps.KindRegistry}
	sw := &Software{EnumerateRegistry: func() ([]apps.App, error) { return []apps.App{app}, nil }}
	if sw.AwaitRemoval(context.Background(), app, 20*time.Millisecond, time.Millisecond) {
		t.Fatal("an app still in the registry was reported as removed")
	}
}

func TestAwaitRemovalTreatsUnreadableRegistryAsInstalled(t *testing.T) {
	sw := &Software{EnumerateRegistry: func() ([]apps.App, error) { return nil, errors.New("access denied") }}
	if sw.AwaitRemoval(context.Background(), apps.App{ID: "x"}, 0, time.Millisecond) {
		t.Fatal("an unreadable registry must not count as removed")
	}
}

func TestLeftoversMeasuredOnce(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Visual Studio Code")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	calls := 0
	sw := &Software{
		Home: root, Local: root, Roaming: root, ProgramData: filepath.Join(root, "missing"),
		Measure: func(string) int64 { calls++; return 4096 },
	}
	got := sw.Leftovers(apps.App{Name: "Visual Studio Code"})
	if len(got) != 1 {
		t.Fatalf("leftovers = %+v", got)
	}
	if got[0].Bytes != 4096 {
		t.Errorf("bytes = %d, want 4096", got[0].Bytes)
	}
	if calls != 1 {
		t.Errorf("measure called %d times, want once", calls)
	}
}
