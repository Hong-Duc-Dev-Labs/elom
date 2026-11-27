package oplog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendAndRead(t *testing.T) {
	dir := t.TempDir()
	log, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := log.RequireWritable(); err != nil {
		t.Fatalf("RequireWritable: %v", err)
	}
	want := []Record{
		{Op: "clean", Path: `C:\cache\a`, Bytes: 10, Result: ResultTrashed},
		{Op: "clean", Path: `C:\cache\b`, Bytes: 0, Result: ResultFailed, Err: "in use"},
	}
	for i, r := range want {
		r.RunID = "run-1"
		if err := log.Append(r); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	got, err := Read(log.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Path != want[i].Path || got[i].Bytes != want[i].Bytes || got[i].Result != want[i].Result || got[i].Err != want[i].Err {
			t.Errorf("record %d = %+v, want %+v", i, got[i], want[i])
		}
		if got[i].Time.IsZero() {
			t.Errorf("record %d has no time", i)
		}
	}
}

func TestReadMissing(t *testing.T) {
	got, err := Read(filepath.Join(t.TempDir(), "none.log"))
	if err != nil || got != nil {
		t.Fatalf("Read(missing) = %v, %v", got, err)
	}
}

func TestReadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.log")
	body := `{"op":"clean","path":"a","result":"trashed"}` + "\n" + `not json` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("Read(corrupt) = %v, want line 2 error", err)
	}
	if len(got) != 1 {
		t.Fatalf("read %d good records, want 1", len(got))
	}
}

func TestRequireWritableFails(t *testing.T) {
	dir := t.TempDir()
	log, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// A directory where the log file should be makes every write fail.
	if err := os.Mkdir(log.Path(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := log.RequireWritable(); err == nil {
		t.Fatal("RequireWritable succeeded on an unwritable log")
	}
}

func TestWriterReusesOneHandle(t *testing.T) {
	log, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writer, err := log.Run()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := writer.Append(Record{Op: "clean", Path: "p", Result: ResultTrashed}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	records, err := Read(log.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %d, want 3", len(records))
	}
	for _, r := range records {
		if r.Time.IsZero() {
			t.Error("writer left a record without a time")
		}
	}
}

func TestRunRefusesUnwritableLog(t *testing.T) {
	log, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(log.Path(), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := log.Run(); err == nil {
		t.Fatal("Run succeeded on an unwritable log")
	}
}

func TestFilterAndSummarize(t *testing.T) {
	records := []Record{
		{Op: "clean", Path: `C:\cache\a`, Bytes: 1000, Result: ResultTrashed},
		{Op: "clean", Path: `C:\cache\b`, Bytes: 2500, Result: ResultPermanent},
		{Op: "clean", Path: `C:\cache\c`, Bytes: 9000, Result: ResultFailed, Err: "locked"},
		{Op: "dns.flush", Path: "dns.flush", Bytes: 0, Result: ResultDone},
		{Op: "uninstall", Path: `C:\Users\me\AppData\Local\OldApp`, Bytes: 500, Result: ResultTrashed},
	}

	all := Summarize(records)
	if all.TotalRecords != 5 {
		t.Errorf("TotalRecords = %d, want 5", all.TotalRecords)
	}
	if all.ReclaimedBytes != 4000 {
		t.Errorf("ReclaimedBytes = %d, want 4000 (failed deletion must not be counted)", all.ReclaimedBytes)
	}
	if all.ByCategory["clean"] != 3 || all.ByCategory["optimize"] != 1 || all.ByCategory["uninstall"] != 1 {
		t.Errorf("ByCategory = %+v", all.ByCategory)
	}

	if got := Filter(records, "clean"); len(got) != 3 {
		t.Errorf("Filter(clean) = %d, want 3", len(got))
	}
	if got := Filter(records, "optimize"); len(got) != 1 || got[0].Op != "dns.flush" {
		t.Errorf("Filter(optimize) = %+v, want dns.flush", got)
	}
	if got := Filter(records, "uninstall"); len(got) != 1 {
		t.Errorf("Filter(uninstall) = %d, want 1", len(got))
	}
	if got := Filter(records, ""); len(got) != 5 {
		t.Errorf("Filter('') = %d, want 5", len(got))
	}
}
