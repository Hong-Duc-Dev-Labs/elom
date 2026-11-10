package svc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
