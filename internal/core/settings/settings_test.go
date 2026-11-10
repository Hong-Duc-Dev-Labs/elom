package settings

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != Default() {
		t.Fatalf("Load() = %+v, want %+v", got, Default())
	}
	if got.PermanentDelete {
		t.Fatal("permanent deletion must be off by default")
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{PermanentDelete: true, MaxScanDepth: 4, MaxScanEntries: 5000, Locale: "vi"}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
	// No temporary files left behind by the atomic write.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "settings.json" {
		t.Fatalf("directory holds %v, want only settings.json", names(entries))
	}
}

func TestZeroLimitsUseDefaults(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(Settings{}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxScanDepth != DefaultMaxDepth || got.MaxScanEntries != DefaultMaxEntries {
		t.Fatalf("limits = %d/%d, want defaults", got.MaxScanDepth, got.MaxScanEntries)
	}
}

func TestUnknownFieldReported(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"permanentDelete":true,"typo":1}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("Load accepted an unknown field")
	}
}

func TestWhitelist(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.Whitelist(); err != nil || got != nil {
		t.Fatalf("Whitelist(missing) = %v, %v", got, err)
	}
	want := []string{`C:\Users\ada\AppData\Local\Keep`, `C:\cache\*.bin`}
	if err := store.SaveWhitelist(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Whitelist()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Whitelist() = %v, want %v", got, want)
	}
}

func TestWhitelistCorruptIsUnreadable(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "whitelist.json"), []byte("[oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = store.Whitelist()
	if err == nil || !errors.Is(err, ErrUnreadable) {
		t.Fatalf("Whitelist(corrupt) = %v, want ErrUnreadable", err)
	}
}

func names(entries []os.DirEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}
