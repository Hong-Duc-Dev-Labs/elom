package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"elom/internal/core/metrics"
	"elom/internal/core/oplog"
	"elom/internal/core/rules"
	"elom/internal/core/safety"
	"elom/internal/core/scan"
)

type fakeStatusReader struct {
	raws []metrics.Raw
	i    int
}

func (f *fakeStatusReader) Read(context.Context) (metrics.Raw, error) {
	if f.i >= len(f.raws) {
		return f.raws[len(f.raws)-1], nil
	}
	r := f.raws[f.i]
	f.i++
	return r, nil
}

func TestIsCommand(t *testing.T) {
	for _, cmd := range []string{"status", "analyze", "history", "clean", "help", "--help", "-h"} {
		if !IsCommand([]string{cmd}) {
			t.Errorf("IsCommand(%q) = false, want true", cmd)
		}
	}
	for _, args := range [][]string{nil, {}, {"--elevated-job", "x"}, {"--unknown"}} {
		if IsCommand(args) {
			t.Errorf("IsCommand(%v) = true, want false", args)
		}
	}
}

func TestStatusJSON(t *testing.T) {
	reader := &fakeStatusReader{raws: []metrics.Raw{
		{CPUPercent: 15, MemUsed: 4_000_000_000, MemTotal: 16_000_000_000, DiskRead: 1000, NetRecv: 500, Uptime: time.Hour},
		{CPUPercent: 25, MemUsed: 4_500_000_000, MemTotal: 16_000_000_000, DiskRead: 2000, NetRecv: 1500, Uptime: time.Hour,
			Processes: []metrics.Process{{PID: 10, Name: "go.exe", CPU: 12.5, Memory: 100_000_000}}},
	}}
	var out, errBuf bytes.Buffer
	code := RunWithDeps(context.Background(), []string{"status", "--json"}, &out, &errBuf, Deps{
		StatusReader:   reader,
		SampleInterval: 5 * time.Millisecond,
	})
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, errBuf.String())
	}
	var got statusOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if got.CPU != 25 || got.MemTotal != 16_000_000_000 || len(got.Processes) != 1 {
		t.Errorf("unexpected status output: %+v", got)
	}
	if got.Health <= 0 || got.Diagnosis.Kind != "ok" {
		t.Errorf("unexpected health/diagnosis: %d / %+v", got.Health, got.Diagnosis)
	}
}

func TestAnalyzeJSON(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "cache")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "blob.bin"), bytes.Repeat([]byte("a"), 2048), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify both flag-before-path and path-before-flag work.
	for _, args := range [][]string{
		{"analyze", "--json", dir},
		{"analyze", dir, "--json"},
	} {
		var out, errBuf bytes.Buffer
		code := RunWithDeps(context.Background(), args, &out, &errBuf, Deps{})
		if code != 0 {
			t.Fatalf("args=%v exit %d, stderr=%s", args, code, errBuf.String())
		}
		var got analyzeNode
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("invalid json: %v\n%s", err, out.String())
		}
		if got.Bytes != 2048 || got.Files != 1 || len(got.Children) != 1 {
			t.Errorf("unexpected analyze output: %+v", got)
		}
	}
}

func TestAnalyzeRequiresPath(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := RunWithDeps(context.Background(), []string{"analyze", "--json"}, &out, &errBuf, Deps{}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestHistoryJSONLimitAndFilter(t *testing.T) {
	dir := t.TempDir()
	log, err := oplog.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []oplog.Record{
		{Op: "clean", Path: `C:\cache\1`, Bytes: 100, Result: oplog.ResultTrashed},
		{Op: "optimize", Path: "dns.flush", Bytes: 0, Result: oplog.ResultDone},
		{Op: "clean", Path: `C:\cache\2`, Bytes: 250, Result: oplog.ResultPermanent},
		{Op: "clean", Path: `C:\cache\3`, Bytes: 400, Result: oplog.ResultFailed, Err: "locked"},
	} {
		if err := log.Append(r); err != nil {
			t.Fatal(err)
		}
	}

	var out, errBuf bytes.Buffer
	code := RunWithDeps(context.Background(), []string{"history", "--json", "--op", "clean", "--limit", "2"}, &out, &errBuf, Deps{
		DataDir: dir,
	})
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, errBuf.String())
	}
	var got historyOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if got.TotalRecords != 3 {
		t.Errorf("TotalRecords = %d, want 3", got.TotalRecords)
	}
	if got.ReclaimedBytes != 350 {
		t.Errorf("ReclaimedBytes = %d, want 350", got.ReclaimedBytes)
	}
	if len(got.Records) != 2 || got.Records[0].Path != `C:\cache\2` || got.Records[1].Path != `C:\cache\3` {
		t.Errorf("Records = %+v, want last 2 clean records", got.Records)
	}
}

func TestCleanRefusesWithoutDryRun(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := RunWithDeps(context.Background(), []string{"clean", "--json"}, &out, &errBuf, Deps{
		DataDir: t.TempDir(),
	})
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "--dry-run") {
		t.Errorf("stderr = %q, want mention of --dry-run", errBuf.String())
	}
}

func TestCleanDryRunJSON(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, "demo-cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "item.dat"), bytes.Repeat([]byte("x"), 512), 0o644); err != nil {
		t.Fatal(err)
	}

	guard := safety.ForRoots(root)
	catalog := []rules.Rule{
		{
			ID:             "demo.cache",
			Category:       "app-cache",
			App:            "Demo",
			Paths:          []string{cacheDir},
			DefaultChecked: true,
		},
	}

	var out, errBuf bytes.Buffer
	code := RunWithDeps(context.Background(), []string{"clean", "--dry-run", "--json"}, &out, &errBuf, Deps{
		DataDir:     t.TempDir(),
		Guard:       guard,
		Catalog:     catalog,
		ScanOptions: scan.Options{RunningProcesses: map[string]bool{}},
	})
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, errBuf.String())
	}
	var got cleanOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if !got.DryRun || got.CandidateCount != 1 || got.TotalBytes != 512 {
		t.Errorf("unexpected clean output: %+v", got)
	}
	// Verify the file was NOT deleted.
	if _, err := os.Stat(filepath.Join(cacheDir, "item.dat")); err != nil {
		t.Fatalf("dry-run deleted file: %v", err)
	}
}
