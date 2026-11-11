//go:build windows

package apps

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A vendor string with a quoted path containing spaces and a quoted argument
// must reach the program unchanged.
func TestRunUninstallKeepsVendorQuotes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Program Files", "Vendor App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "unins 000.bat")
	if err := os.WriteFile(script, []byte("@echo off\r\necho ran %~1 %2\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var lines []string
	command := `"` + script + `" "/arg x" /S`
	if err := RunUninstall(context.Background(), command, func(line string) { lines = append(lines, line) }); err != nil {
		t.Fatalf("RunUninstall(%s): %v; output %q", command, err, lines)
	}
	got := strings.Join(lines, "\n")
	if !strings.Contains(got, "ran /arg x /S") {
		t.Fatalf("output %q, want the script to run with its arguments", got)
	}
}

func TestRunUninstallReportsFailure(t *testing.T) {
	err := RunUninstall(context.Background(), `"C:\no such folder\unins.exe" /S`, nil)
	if err == nil {
		t.Fatal("a missing uninstaller must fail")
	}
}
