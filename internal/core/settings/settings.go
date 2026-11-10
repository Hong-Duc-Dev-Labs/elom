// Package settings stores the user's preferences and the list of paths the
// cleaner must never touch.
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Default scan limits. They bound a walk so a pathological folder cannot
// exhaust memory, and keep the first Analyze of a large profile quick: an
// index that stops at the cap reports partial sizes instead of freezing the
// window for minutes.
const (
	DefaultMaxDepth   = 12
	DefaultMaxEntries = 300_000
)

// Settings is the file's contents. Unknown keys are preserved by ignoring
// nothing: decoding is strict, so a typo is reported instead of silently
// dropped.
type Settings struct {
	PermanentDelete bool   `json:"permanentDelete"`
	MaxScanDepth    int    `json:"maxScanDepth"`
	MaxScanEntries  int    `json:"maxScanEntries"`
	Locale          string `json:"locale"`
}

// Default returns the settings a fresh install starts with. Permanent
// deletion is off: the Recycle Bin is the default.
func Default() Settings {
	return Settings{
		PermanentDelete: false,
		MaxScanDepth:    DefaultMaxDepth,
		MaxScanEntries:  DefaultMaxEntries,
		Locale:          "",
	}
}

// ErrUnreadable wraps every failure to read the whitelist. Cleanup must stop
// when it cannot prove which paths the user protects.
var ErrUnreadable = errors.New("settings: whitelist unreadable")

// Store reads and writes the settings directory.
type Store struct {
	dir string
}

// Open prepares the store at dir, creating it when needed.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Dir returns the settings directory.
func (s *Store) Dir() string { return s.dir }

// Load returns the stored settings, or the defaults when no file exists.
func (s *Store) Load() (Settings, error) {
	path := filepath.Join(s.dir, "settings.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), err
	}
	got := Default()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); err != nil {
		return Default(), fmt.Errorf("settings: %s: %w", path, err)
	}
	return got.normalized(), nil
}

// Save writes the settings atomically: a crash leaves either the old file or
// the new one, never half of either.
func (s *Store) Save(v Settings) error {
	v = v.normalized()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.dir, "settings.json"), append(data, '\n'))
}

// Whitelist returns the protected glob patterns. A missing file means no
// patterns; an unreadable or corrupt file is ErrUnreadable, and cleanup must
// refuse to run.
func (s *Store) Whitelist() ([]string, error) {
	path := filepath.Join(s.dir, "whitelist.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	var patterns []string
	if err := json.Unmarshal(data, &patterns); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return patterns, nil
}

// SaveWhitelist stores the patterns atomically.
func (s *Store) SaveWhitelist(patterns []string) error {
	if patterns == nil {
		patterns = []string{}
	}
	data, err := json.MarshalIndent(patterns, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.dir, "whitelist.json"), append(data, '\n'))
}

func (v Settings) normalized() Settings {
	if v.MaxScanDepth <= 0 {
		v.MaxScanDepth = DefaultMaxDepth
	}
	if v.MaxScanEntries <= 0 {
		v.MaxScanEntries = DefaultMaxEntries
	}
	return v
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(name)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
