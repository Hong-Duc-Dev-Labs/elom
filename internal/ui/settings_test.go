package ui

import (
	"strings"
	"testing"
	"time"

	. "github.com/egoist/mygo/ui"

	"elom/locales"
)

func TestSettingsShowsLanguageWhitelistAndLog(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "settings"
	a.whitelist = []string{`C:\Users\me\AppData\Local\Keep`}
	a.newPattern = `C:\cache\*.bin`

	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"Language", "PROTECTION LIST", `C:\Users\me\AppData\Local\Keep`, "Clean screen", "RECENT OPERATIONS", "Nothing recorded yet."} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
}

func TestWhitelistEditorRoundTrip(t *testing.T) {
	a := newTestApp(t)
	a.newPattern = `C:\Keep\this`
	a.addWhitelistPattern()
	if len(a.whitelist) != 1 || a.whitelist[0] != `C:\Keep\this` {
		t.Fatalf("whitelist = %v", a.whitelist)
	}
	if a.newPattern != "" {
		t.Errorf("editor not cleared: %q", a.newPattern)
	}
	saved, err := a.Store.Whitelist()
	if err != nil || len(saved) != 1 || saved[0] != `C:\Keep\this` {
		t.Fatalf("saved = %v, %v", saved, err)
	}

	// Duplicates and blanks are ignored.
	a.newPattern = `  `
	a.addWhitelistPattern()
	a.newPattern = `C:\KEEP\this`
	a.addWhitelistPattern()
	if len(a.whitelist) != 1 {
		t.Errorf("whitelist grew to %v", a.whitelist)
	}

	a.removeWhitelistPattern(0)
	if len(a.whitelist) != 0 {
		t.Fatalf("whitelist = %v", a.whitelist)
	}
	saved, _ = a.Store.Whitelist()
	if len(saved) != 0 {
		t.Errorf("saved = %v", saved)
	}
}

func TestCleanScreenView(t *testing.T) {
	a := newTestApp(t)
	a.cleanOpenedAt = time.Now().Add(-5 * time.Second)
	tt := NewTester(a.cleanScreenView, 800, 600)
	if !tt.HasText("Press Esc") {
		t.Errorf("hint missing: %q", tt.Texts())
	}
	// A click after the guard period dismisses the cover; with no window it
	// must simply not panic.
	tt.ClickAt(400, 300)
}

func TestVietnameseCatalogResolves(t *testing.T) {
	vi := locales.In("vi")
	if got := vi.NavClean(); got != "Dọn dẹp" {
		t.Errorf("navClean = %q", got)
	}
	if got := vi.CleanConfirmTitle(3); !strings.Contains(got, "3") {
		t.Errorf("plural message lost its count: %q", got)
	}
	if got := locales.In("en").NavClean(); got != "Clean" {
		t.Errorf("English navClean = %q", got)
	}
}
