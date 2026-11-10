package ui

import (
	"errors"
	"strings"
	"testing"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/scan"
)

func TestCleanPageRendersCandidates(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "clean"
	a.clean.scanned = true
	a.clean.reviewing = true
	a.clean.candidates = []scan.Candidate{
		{Path: `C:\Users\ada\AppData\Local\Chrome\Cache`, Bytes: 2048, Category: "browser", Note: "Chrome rebuilds this.", Checked: true},
		{Path: `C:\Users\ada\AppData\Local\Kept`, Bytes: 10, Category: "logs", Blocked: errors.New("protected by your list")},
	}
	a.clean.checked[`C:\Users\ada\AppData\Local\Chrome\Cache`] = true

	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{
		`C:\Users\ada\AppData\Local\Chrome\Cache`,
		"2.0 kB",
		"Chrome rebuilds this.",
		"protected by your list",
	} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
	if !strings.Contains(texts, "items selected") {
		t.Errorf("summary missing:\n%s", texts)
	}
}

func TestCleanPageStartsEmpty(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "clean"
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "Scan for files to remove") {
		t.Errorf("no scan button:\n%s", texts)
	}
	if strings.Contains(texts, "items selected") {
		t.Errorf("summary shown before a scan:\n%s", texts)
	}
}

func TestCleanPageShowsDisabledReason(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "clean"
	a.clean.err = "settings: whitelist unreadable"
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "Cleanup is disabled") {
		t.Errorf("disabled state not shown:\n%s", texts)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		0:             "0 B",
		999:           "999 B",
		1000:          "1.0 kB",
		2048:          "2.0 kB",
		1_500_000:     "1.5 MB",
		7_500_000_000: "7.5 GB",
	}
	for in, want := range cases {
		if got := humanBytes(in); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

// The path belongs to the row; the checkbox carries it as its accessible
// name instead of a visible caption, which used to print every path twice.
func TestCleanRowHasAccessibleCheckbox(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "clean"
	a.clean.scanned = true
	a.clean.reviewing = true
	path := `C:\Users\ada\AppData\Local\Temp`
	a.clean.candidates = []scan.Candidate{{Path: path, Bytes: 2048, Category: "temp"}}

	tt := NewTester(a.View, 1180, 760)
	if _, ok := tt.AccessByRole(RoleCheckBox, path); !ok {
		t.Errorf("the row checkbox has no accessible name %q", path)
	}
}
