package svc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"elom/internal/core/oplog"
	"elom/internal/core/rules"
	"elom/internal/core/safety"
	"elom/internal/core/scan"
	"elom/internal/core/settings"
)

type fakeTrasher struct{ done []string }

func (f *fakeTrasher) Trash(path string) error {
	f.done = append(f.done, path)
	return nil
}

func newCleanForTest(t *testing.T, root string, catalog []rules.Rule) (*Clean, *settings.Store, *oplog.Log, *fakeTrasher) {
	t.Helper()
	store, err := settings.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	log, err := oplog.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	trash := &fakeTrasher{}
	return NewClean(safety.ForRoots(root), store, log, catalog, trash), store, log, trash
}

func TestScanAndDelete(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Browser", "Cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blob.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog := []rules.Rule{{
		ID: "test-cache", Category: "browser", App: "Test",
		Paths: []string{dir}, DefaultChecked: true, Regenerates: true,
	}}
	clean, _, log, trash := newCleanForTest(t, root, catalog)

	got, err := clean.Scan(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Bytes != 2048 || got[0].Blocked != nil {
		t.Fatalf("scan = %+v", got)
	}
	if !got[0].Checked {
		t.Error("default-checked rule produced an unchecked candidate")
	}

	results := clean.Delete(context.Background(), got, false)
	if len(results) != 1 || results[0].Err != "" {
		t.Fatalf("delete = %+v", results)
	}
	if len(trash.done) != 1 || trash.done[0] != dir {
		t.Fatalf("trash = %v", trash.done)
	}
	records, err := oplog.Read(log.Path())
	if err != nil || len(records) != 1 || records[0].Path != dir {
		t.Fatalf("oplog = %+v, %v", records, err)
	}
}

func TestScanRefusesCorruptWhitelist(t *testing.T) {
	root := t.TempDir()
	clean, store, _, _ := newCleanForTest(t, root, []rules.Rule{{ID: "x", Category: "logs", Paths: []string{root}}})
	if err := os.WriteFile(filepath.Join(store.Dir(), "whitelist.json"), []byte("[oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := clean.Scan(context.Background(), nil); err == nil {
		t.Fatal("Scan ran with an unreadable protection list")
	}
}

func TestScanRefusesUnwritableLog(t *testing.T) {
	root := t.TempDir()
	clean, _, log, _ := newCleanForTest(t, root, []rules.Rule{{ID: "x", Category: "logs", Paths: []string{root}}})
	if err := os.Mkdir(log.Path(), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := clean.Scan(context.Background(), nil); err == nil {
		t.Fatal("Scan ran with an unwritable operation log")
	}
}

func TestDeleteSkipsBlocked(t *testing.T) {
	root := t.TempDir()
	clean, _, _, trash := newCleanForTest(t, root, nil)
	got := clean.Delete(context.Background(), []scan.Candidate{
		{Path: filepath.Join(root, "kept"), Blocked: errors.New("refused")},
	}, false)
	if len(got) != 0 {
		t.Fatalf("delete returned %+v", got)
	}
	if len(trash.done) != 0 {
		t.Fatalf("trash = %v", trash.done)
	}
}

// TestRealScan is opt-in: it measures and sizes the real catalog on this PC,
// which takes seconds and touches many folders. Set ELOM_REAL_SCAN=1.
func TestRealScan(t *testing.T) {
	if os.Getenv("ELOM_REAL_SCAN") == "" {
		t.Skip("set ELOM_REAL_SCAN=1 to scan the real catalog")
	}
	clean, _, _, _ := newCleanForTest(t, `C:\Users`, nil)
	guard := safety.New()
	_ = guard
	clean.Guard = guard
	start := time.Now()
	got, err := clean.Scan(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	var blocked int
	for _, c := range got {
		total += c.Bytes
		if c.Blocked != nil {
			blocked++
		}
	}
	t.Logf("%d candidates, %s reclaimable, %d blocked, %s", len(got), humanBytes(total), blocked, time.Since(start).Round(time.Millisecond))
}

func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}
