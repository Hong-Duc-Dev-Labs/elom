package ui

import (
	"strings"
	"testing"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/installer"
)

func TestInstallersTabRendersCandidates(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "clean"
	a.cleanTab = 2
	a.installers.scanned = true
	a.installers.candidates = []installer.Candidate{
		{
			Path:    `C:\Users\ada\Downloads\app-setup.msi`,
			Name:    "app-setup.msi",
			Ext:     ".msi",
			Bytes:   12_000_000,
			AgeDays: 45,
			Checked: true,
		},
		{
			Path:    `C:\Users\ada\Downloads\tool.exe`,
			Name:    "tool.exe",
			Ext:     ".exe",
			Bytes:   3_000_000,
			AgeDays: 20,
			Checked: false,
		},
	}
	a.installers.checked[`C:\Users\ada\Downloads\app-setup.msi`] = true

	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{
		"Installers",
		"OLD INSTALLERS IN DOWNLOADS",
		"app-setup.msi",
		"45d old",
		"12.0 MB",
		"tool.exe",
		"portable tools",
		"1 items selected",
	} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
}
