package svc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"elom/internal/core/oplog"
	"elom/internal/core/settings"
)

func TestPurgeScanAndDelete(t *testing.T) {
	root := t.TempDir()
	old := time.Now().Add(-15 * 24 * time.Hour)
	proj := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(proj, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "package.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "node_modules", "pkg", "a.js"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(filepath.Join(proj, "package.json"), old, old)
	_ = os.Chtimes(filepath.Join(proj, "node_modules"), old, old)

	store, err := settings.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	log, err := oplog.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	trash := &fakeTrasher{}
	p := NewPurge(store, log, trash)

	if err := p.SavePaths([]string{root}); err != nil {
		t.Fatal(err)
	}
	paths, err := p.Paths()
	if err != nil || len(paths) != 1 || paths[0] != root {
		t.Fatalf("Paths = %v, %v", paths, err)
	}

	found, err := p.Scan(context.Background(), paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Artifact != "node_modules" || !found[0].Checked {
		t.Fatalf("Scan = %+v", found)
	}

	results := p.Delete(context.Background(), paths, found)
	if len(results) != 1 || results[0].Err != "" {
		t.Fatalf("Delete = %+v", results)
	}
	if len(trash.done) != 1 || trash.done[0] != found[0].Path {
		t.Fatalf("trash = %v", trash.done)
	}
	records, err := oplog.Read(log.Path())
	if err != nil || len(records) != 1 || records[0].Result != oplog.ResultTrashed {
		t.Fatalf("oplog = %+v, %v", records, err)
	}
}
