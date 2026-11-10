package ui

import (
	"testing"

	. "github.com/egoist/mygo/ui"
)

// The window must draw Geist even on a machine that has never installed it:
// New registers the embedded files, and this pins that they are usable.
func TestGeistFamiliesAreRegistered(t *testing.T) {
	newTestApp(t)

	seen := map[string]bool{}
	for _, f := range FontFamilies() {
		switch f.Name {
		case "Geist", "Geist Mono":
			seen[f.Name] = true
			if !f.Registered {
				t.Errorf("%s is listed but not registered by the app", f.Name)
			}
			if f.Name == "Geist Mono" && !f.Monospaced {
				t.Errorf("Geist Mono is not reported as monospaced")
			}
		}
	}
	for _, name := range []string{"Geist", "Geist Mono"} {
		if !seen[name] {
			t.Errorf("%s is not registered", name)
		}
	}
}
