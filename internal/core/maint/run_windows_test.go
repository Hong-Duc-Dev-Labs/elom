//go:build windows

package maint

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
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

func TestSummarizeToolOutput(t *testing.T) {
	// Simulate UTF-16LE sfc.exe output with header boilerplate and conclusion.
	sfcText := "Beginning system scan.  This process will take some time.\r\n\r\nBeginning verification phase of system scan.\r\nVerification 100% complete.\r\n\r\nWindows Resource Protection did not find any integrity violations.\r\n"
	encoded := utf16.Encode([]rune(sfcText))
	raw := make([]byte, 2+len(encoded)*2)
	raw[0], raw[1] = 0xFF, 0xFE
	for i, r := range encoded {
		binary.LittleEndian.PutUint16(raw[2+i*2:], r)
	}
	if got := summarizeToolOutput(raw); !strings.Contains(got, "did not find any integrity violations") {
		t.Errorf("summarizeToolOutput(sfc) = %q", got)
	}

	dismText := "Deployment Image Servicing and Management tool\r\nVersion: 10.0.26100.1\r\n\r\nImage Version: 10.0.26100.1\r\n\r\nNo component store corruption detected.\r\nThe operation completed successfully.\r\n"
	if got := summarizeToolOutput([]byte(dismText)); got != "The operation completed successfully." {
		t.Errorf("summarizeToolOutput(dism) = %q", got)
	}
}

func TestParseSCQueryRunning(t *testing.T) {
	running := "SERVICE_NAME: wuauserv\r\n        TYPE               : 30  WIN32\r\n        STATE              : 4  RUNNING\r\n"
	stopped := "SERVICE_NAME: wuauserv\r\n        TYPE               : 30  WIN32\r\n        STATE              : 1  STOPPED\r\n"
	if !parseSCQueryRunning(running) {
		t.Error("expected RUNNING to return true")
	}
	if parseSCQueryRunning(stopped) {
		t.Error("expected STOPPED to return false")
	}
}

func TestParsePhysicalDisks(t *testing.T) {
	single := []byte(`{"FriendlyName":"Samsung SSD 980","MediaType":"SSD","HealthStatus":"Healthy"}`)
	got, err := parsePhysicalDisks(single)
	if err != nil || got != "Samsung SSD 980 (SSD): Healthy" {
		t.Fatalf("single = %q, err = %v", got, err)
	}
	multi := []byte(`[{"FriendlyName":"NVMe0","MediaType":"SSD","HealthStatus":"Healthy"},{"FriendlyName":"Backup","MediaType":"HDD","HealthStatus":"Warning"}]`)
	got, err = parsePhysicalDisks(multi)
	if err != nil || !strings.Contains(got, "NVMe0 (SSD): Healthy") || !strings.Contains(got, "Backup (HDD): Warning") {
		t.Fatalf("multi = %q, err = %v", got, err)
	}
}

func TestClearOldFilesInDirs(t *testing.T) {
	dir := t.TempDir()
	oldFile := filepath.Join(dir, "old.log")
	newFile := filepath.Join(dir, "new.log")
	if err := os.WriteFile(oldFile, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	removed := clearOldFilesInDirs([]string{dir}, time.Now().Add(-7*24*time.Hour))
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old.log was not removed")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Error("new.log was unexpectedly removed")
	}
}

func validArgs(op string) map[string]string {
	if op == "startup.set-hklm" {
		return map[string]string{"name": "Probe", "enabled": "false", "scope": "machine"}
	}
	return nil
}
