package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFileAge(t *testing.T, path string, size int, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsOldInstallersLevels1And2(t *testing.T) {
	downloads := t.TempDir()
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	old := now.Add(-30 * 24 * time.Hour)
	fresh := now.Add(-3 * 24 * time.Hour)

	// Level 1: old .msi (checked), old .exe (unchecked), fresh .iso (skipped), old .pdf (skipped).
	writeFileAge(t, filepath.Join(downloads, "app-setup.msi"), 5000, old)
	writeFileAge(t, filepath.Join(downloads, "portable-tool.exe"), 3000, old)
	writeFileAge(t, filepath.Join(downloads, "recent.iso"), 9000, fresh)
	writeFileAge(t, filepath.Join(downloads, "invoice.pdf"), 4000, old)

	// Level 2: old .iso inside a subfolder (found & checked).
	writeFileAge(t, filepath.Join(downloads, "isos", "linux.iso"), 8000, old)

	// Level 3: too deep (must be ignored).
	writeFileAge(t, filepath.Join(downloads, "a", "b", "deep.msi"), 7000, old)

	got := Scan(context.Background(), downloads, Options{
		Now: func() time.Time { return now },
	})
	if len(got) != 3 {
		t.Fatalf("got %d candidates, want 3: %+v", len(got), got)
	}

	// Sorted largest first: linux.iso (8000), app-setup.msi (5000), portable-tool.exe (3000).
	if got[0].Name != "linux.iso" || !got[0].Checked || got[0].AgeDays != 30 {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Name != "app-setup.msi" || !got[1].Checked {
		t.Errorf("got[1] = %+v", got[1])
	}
	if got[2].Name != "portable-tool.exe" || got[2].Checked || got[2].Blocked != nil {
		t.Errorf("got[2] (.exe) should be unchecked by default and not blocked: %+v", got[2])
	}
}

func TestScanRespectsWhitelist(t *testing.T) {
	downloads := t.TempDir()
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	old := now.Add(-20 * 24 * time.Hour)
	keep := filepath.Join(downloads, "keep.msi")
	writeFileAge(t, keep, 2048, old)

	got := Scan(context.Background(), downloads, Options{
		Now:       func() time.Time { return now },
		Whitelist: []string{keep},
	})
	if len(got) != 1 || !errors.Is(got[0].Blocked, ErrWhitelisted) || got[0].Checked {
		t.Fatalf("whitelisted installer should be blocked: %+v", got)
	}
}
