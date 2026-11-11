//go:build windows

package maint

import (
	"context"
	"os"
	"testing"
)

// TestRefusesAdminWhenUnelevated proves the real op table cannot run a
// privileged operation from an unelevated process, whatever file asked.
func TestRefusesAdminWhenUnelevated(t *testing.T) {
	if isElevated() {
		t.Skip("this shell already has administrator rights")
	}
	for _, task := range Tasks {
		if !task.Admin {
			continue
		}
		result := Run(context.Background(), Op{Name: task.Op, Args: validArgs(task.Op)})
		if result.Err != ErrNeedsAdmin.Error() {
			t.Errorf("%s ran unelevated: %+v", task.Op, result)
		}
	}
}

func TestRunJobRefusesBadFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + `\job.json`
	if err := os.WriteFile(path, []byte(`{"v":1,"ops":[{"op":"format.c"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RunJob(path); err == nil {
		t.Fatal("RunJob accepted an unknown op")
	}
	if _, err := os.Stat(ResultPath(path)); err == nil {
		t.Fatal("RunJob wrote results for a refused job")
	}
}

func TestWriteResultReplacesWholeFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + `\job.json.result.json`
	if err := os.WriteFile(path, []byte("old and longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeResult(path, []byte(`[]`)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `[]` {
		t.Fatalf("result file = %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("%d files beside the result, want only the result", len(entries))
	}
}

func validArgs(op string) map[string]string {
	if op == "startup.set-hklm" {
		return map[string]string{"name": "Probe", "enabled": "false", "scope": "machine"}
	}
	return nil
}
