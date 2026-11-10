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

func TestSettingsFitsTheDefaultWindow(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "settings"
	// The window is configured 1180x760; the whole page, recent operations
	// last, must be on screen without the page scroll.
	tt := NewTester(a.View, 1180, 760)
	for _, want := range []string{"RECENT OPERATIONS", locales.LogEmpty(), "dev"} {
		r, ok := tt.Find(want)
		if !ok {
			t.Fatalf("%q is missing", want)
		}
		if r.Y+r.H > 760 {
			t.Errorf("%q ends at %.1f, below the 760 DIP window", want, r.Y+r.H)
		}
	}
}

// A long pattern was 1096 DIP of unbreakable mono text: it widened its
// column past the half, pushed Protected folders off the window, and put the
// Remove button out of reach. The row must truncate instead.
func TestSettingsLongPatternKeepsTheColumns(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "settings"
	a.whitelist = []string{`C:\` + strings.Repeat(`deeply-nested-folder\`, 8) + `file.bin`}
	tt := settingsColumnsStayInsideThePage(t, a)
	if _, ok := tt.Access(locales.WhitelistRemove()); !ok {
		t.Error("the long pattern's Remove button is missing")
	}
}

// The allow roots come from the user's profile; a long one must truncate in
// place rather than grow the right column and squeeze the left one.
func TestSettingsLongProtectedRootKeepsTheColumns(t *testing.T) {
	home := `C:\Users\` + strings.Repeat("a-very-long-user-name-", 5)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", home+`\AppData\Local`)
	t.Setenv("APPDATA", home+`\AppData\Roaming`)
	a := newTestApp(t)
	a.Screen = "settings"
	tt := settingsColumnsStayInsideThePage(t, a)
	if _, ok := tt.Find(home + `\AppData\Local`); !ok {
		t.Error("the long allow root is not drawn")
	}
}

// settingsColumnsStayInsideThePage fails unless both column headers keep
// their equal halves and every control stays inside the 1100 DIP page
// (x 40..1140 in the 1180 DIP window).
func settingsColumnsStayInsideThePage(t *testing.T, a *App) *Tester {
	t.Helper()
	tt := NewTester(a.View, 1180, 760)
	left, ok := tt.Find("PREFERENCES")
	if !ok || left.W == 0 {
		t.Fatalf("left column header missing: %+v", left)
	}
	right, ok := tt.Find("PROTECTED FOLDERS")
	if !ok || right.W == 0 {
		t.Fatalf("right column header missing: %+v", right)
	}
	if diff := left.W - right.W; diff > 1 || diff < -1 {
		t.Errorf("columns are uneven: left %.1f, right %.1f", left.W, right.W)
	}
	if right.X+right.W > 1140.5 {
		t.Errorf("right column ends at %.1f, outside the page", right.X+right.W)
	}
	for _, control := range []string{locales.WhitelistAdd(), locales.WhitelistRemove()} {
		if info, ok := tt.Access(control); ok && info.Bounds.X+info.Bounds.W > 1140 {
			t.Errorf("%q ends at %.1f, off the page", control, info.Bounds.X+info.Bounds.W)
		}
	}
	return tt
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
