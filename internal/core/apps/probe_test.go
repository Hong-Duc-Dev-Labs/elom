package apps

import (
	"context"
	"os"
	"testing"
)

func TestRealApps(t *testing.T) {
	if os.Getenv("ELOM_REAL_APPS") == "" {
		t.Skip("set ELOM_REAL_APPS=1")
	}
	list, err := Enumerate()
	if err != nil {
		t.Fatal(err)
	}
	packaged, err := EnumeratePackaged(context.Background())
	if err != nil {
		t.Logf("packaged: %v", err)
	}
	t.Logf("registry %d, packaged %d", len(list), len(packaged))
	for i, app := range list {
		if i >= 3 {
			break
		}
		t.Logf("  %s | %s | %d bytes", app.Name, app.Version, app.SizeBytes)
	}
}
