package core

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoMyGoInCore is the import boundary described in plans: the packages
// under internal/core must stay usable without the desktop framework, so a
// command line tool can share them. It shells out to the Go tool rather than
// parsing imports by hand, which sees every transitive dependency.
func TestNoMyGoInCore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping go list in short mode")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "list", "-deps", "./internal/core/...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("go list: %v: %s", err, ee.Stderr)
		}
		t.Fatalf("go list: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "github.com/egoist/mygo") {
			t.Fatalf("internal/core depends on MyGo: %s", line)
		}
	}
}
