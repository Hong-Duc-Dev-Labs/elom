package delete

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"elom/internal/core/oplog"
	"elom/internal/core/safety"
)

type fakeTrasher struct {
	done []string
	fail map[string]error
}

func (f *fakeTrasher) Trash(path string) error {
	if err := f.fail[path]; err != nil {
		return err
	}
	f.done = append(f.done, path)
	return nil
}

func fixture(t *testing.T) (root string, guard *safety.Guard, log *oplog.Log, trash *fakeTrasher) {
	t.Helper()
	root = t.TempDir()
	guard = safety.ForRoots(root)
	l, err := oplog.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root, guard, l, &fakeTrasher{fail: map[string]error{}}
}

func write(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunTrashesAndLogs(t *testing.T) {
	root, guard, log, trash := fixture(t)
	path := write(t, filepath.Join(root, "cache", "a.bin"))
	got := Run(context.Background(), guard, log, "run-1", []Item{{Path: path, Bytes: 4, Category: "app-cache"}}, trash, false)

	if len(got) != 1 || got[0].Outcome != oplog.ResultTrashed || got[0].Err != "" {
		t.Fatalf("result = %+v", got)
	}
	if len(trash.done) != 1 || trash.done[0] != path {
		t.Fatalf("trash called with %v", trash.done)
	}
	records, err := oplog.Read(log.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Path != path || records[0].Result != oplog.ResultTrashed || records[0].Bytes != 4 {
		t.Fatalf("log = %+v", records)
	}
	if records[0].RunID != "run-1" {
		t.Errorf("run id = %q", records[0].RunID)
	}
}

func TestRunRefusesOutsideRoot(t *testing.T) {
	_, guard, log, trash := fixture(t)
	outside := write(t, filepath.Join(t.TempDir(), "x.bin"))
	got := Run(context.Background(), guard, log, "run", []Item{{Path: outside}}, trash, false)
	if got[0].Outcome != oplog.ResultFailed || got[0].Err == "" {
		t.Fatalf("result = %+v, want failed", got[0])
	}
	if len(trash.done) != 0 {
		t.Fatalf("trash called for a refused path: %v", trash.done)
	}
}

func TestRunRecordsTrasherFailure(t *testing.T) {
	root, guard, log, trash := fixture(t)
	path := write(t, filepath.Join(root, "locked.bin"))
	trash.fail[path] = errors.New("file is in use")
	got := Run(context.Background(), guard, log, "run", []Item{{Path: path}}, trash, false)
	if got[0].Outcome != oplog.ResultFailed || !strings.Contains(got[0].Err, "in use") {
		t.Fatalf("result = %+v", got[0])
	}
	records, _ := oplog.Read(log.Path())
	if len(records) != 1 || records[0].Result != oplog.ResultFailed {
		t.Fatalf("log = %+v", records)
	}
}

func TestPermanentOnlyForSystemTemp(t *testing.T) {
	root, guard, log, trash := fixture(t)
	cache := write(t, filepath.Join(root, "cache.bin"))
	temp := write(t, filepath.Join(root, "temp.bin"))

	got := Run(context.Background(), guard, log, "run", []Item{
		{Path: cache, Category: "app-cache"},
		{Path: temp, Category: PermanentCategory},
	}, trash, true)

	if got[0].Outcome != oplog.ResultTrashed {
		t.Errorf("app-cache went permanent: %+v", got[0])
	}
	if got[1].Outcome != oplog.ResultPermanent {
		t.Errorf("system-temp was not permanent: %+v", got[1])
	}
	if _, err := os.Stat(temp); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("permanent deletion left %s: %v", temp, err)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Errorf("trashed item was removed from disk: %v", err)
	}
}

func TestUnwritableLogStopsRun(t *testing.T) {
	root, guard, _, trash := fixture(t)
	dir := t.TempDir()
	log, err := oplog.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(log.Path(), 0o755); err != nil {
		t.Fatal(err)
	}
	a := write(t, filepath.Join(root, "a.bin"))
	b := write(t, filepath.Join(root, "b.bin"))
	got := Run(context.Background(), guard, log, "run", []Item{{Path: a}, {Path: b}}, trash, false)
	if len(got) != 2 {
		t.Fatalf("got %d results", len(got))
	}
	for _, r := range got {
		if r.Outcome != oplog.ResultFailed || !strings.Contains(r.Err, ErrLogUnwritable.Error()) {
			t.Errorf("result = %+v, want log failure", r)
		}
	}
	if len(trash.done) != 0 {
		t.Errorf("items trashed without a log: %v", trash.done)
	}
}
