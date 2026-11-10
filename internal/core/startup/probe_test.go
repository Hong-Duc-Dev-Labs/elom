package startup

import (
	"os"
	"testing"
)

func TestRealStartup(t *testing.T) {
	if os.Getenv("ELOM_REAL_STARTUP") == "" {
		t.Skip("set ELOM_REAL_STARTUP=1")
	}
	list, err := Enumerate()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d entries", len(list))
	for i, e := range list {
		if i >= 5 {
			break
		}
		t.Logf("  [%s] %s enabled=%v writable=%v", e.Source, e.Name, e.Enabled, e.Writable)
	}
}
