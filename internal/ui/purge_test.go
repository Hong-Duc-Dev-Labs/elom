package ui

import (
	"strings"
	"testing"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/purge"
)

func TestPurgeTabRendersCandidatesAndRecentTag(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "clean"
	a.cleanTab = 1
	a.purge.paths = []string{`C:\Users\ada\Projects`}
	a.purge.scanned = true
	a.purge.candidates = []purge.Candidate{
		{
			Path:        `C:\Users\ada\Projects\web\node_modules`,
			ProjectDir:  `C:\Users\ada\Projects\web`,
			ProjectName: "web",
			Artifact:    "node_modules",
			Bytes:       5_000_000,
			Checked:     true,
		},
		{
			Path:        `C:\Users\ada\Projects\fresh\target`,
			ProjectDir:  `C:\Users\ada\Projects\fresh`,
			ProjectName: "fresh",
			Artifact:    "target",
			Bytes:       2_000_000,
			Recent:      true,
			Checked:     false,
		},
		{
			Path:        `C:\Users\ada\Projects\tracked\dist`,
			ProjectDir:  `C:\Users\ada\Projects\tracked`,
			ProjectName: "tracked",
			Artifact:    "dist",
			Bytes:       500_000,
			Blocked:     purge.ErrGitTracked,
		},
	}
	a.purge.checked[`C:\Users\ada\Projects\web\node_modules`] = true

	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{
		"Dev Purge",
		"PROJECT FOLDERS",
		"web",
		"node_modules",
		"5.0 MB",
		"fresh",
		"recent",
		"contains git-tracked files",
		"1 items selected",
	} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
}
