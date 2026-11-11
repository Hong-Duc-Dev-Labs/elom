package ui

import (
	"path/filepath"
	"strings"
	"testing"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/delete"
	"elom/internal/core/safety"
	"elom/internal/core/scan"
)

func fakeIndex() *scan.Node {
	root := &scan.Node{Path: `C:\Users\ada`, Name: "ada", Dir: true, Bytes: 6000, Files: 3}
	big := &scan.Node{Path: `C:\Users\ada\big`, Name: "big", Dir: true, Bytes: 4000, Files: 2, Depth: 1}
	big.Children = []*scan.Node{
		{Path: `C:\Users\ada\big\one.bin`, Name: "one.bin", Bytes: 3000, Files: 1, Depth: 2},
		{Path: `C:\Users\ada\big\two.bin`, Name: "two.bin", Bytes: 1000, Files: 1, Depth: 2},
	}
	root.Children = []*scan.Node{
		big,
		{Path: `C:\Users\ada\three.bin`, Name: "three.bin", Bytes: 2000, Files: 1, Depth: 1},
	}
	return root
}

func TestAnalyzeRendersIndex(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "analyze"
	root := fakeIndex()
	a.analyze.root = root
	a.analyze.current = root
	a.analyze.largest = scan.LargestFiles(root, 20)

	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"ada", "6.0 kB", "Map", "Largest files", "big"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}

	a.analyze.view = 1
	tt = NewTester(a.View, 1180, 760)
	if !tt.HasText("one.bin") {
		t.Errorf("largest files not listed: %q", tt.Texts())
	}
}

func TestAnalyzeBreadcrumbGoesBack(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "analyze"
	root := fakeIndex()
	a.analyze.root = root
	a.analyze.current = root
	a.resetSelected()
	a.openChild(0)
	if a.analyze.current == root {
		t.Fatal("openChild(0) did not open the folder")
	}
	tt := NewTester(a.View, 1180, 760)
	if err := tt.Click("ada"); err != nil {
		t.Fatal(err)
	}
	if a.analyze.current != root || len(a.analyze.stack) != 0 {
		t.Errorf("breadcrumb did not return to the root: current %q, stack %d", a.analyze.current.Name, len(a.analyze.stack))
	}
}

func TestAnalyzeDrillDownAndUp(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "analyze"
	root := fakeIndex()
	a.analyze.root = root
	a.analyze.current = root

	a.openChild(0) // big
	if a.analyze.current != root.Children[0] {
		t.Fatalf("current = %v", a.analyze.current.Name)
	}
	if len(a.analyze.stack) != 1 {
		t.Fatalf("stack = %d", len(a.analyze.stack))
	}
	if len(a.analyze.largest) != 2 {
		t.Errorf("largest in big = %d, want 2", len(a.analyze.largest))
	}
	// The first child is a directory, so the stack grew; clicking a file
	// instead selects its row rather than descending.
	a.openChild(0)
	if len(a.analyze.stack) != 1 {
		t.Errorf("descended into a file; stack = %d", len(a.analyze.stack))
	}
}

func TestAnalyzeEmptyState(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "analyze"
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "Analyze your user folder") {
		t.Errorf("no start button:\n%s", texts)
	}
}

func TestTileAt(t *testing.T) {
	tiles := []Tile{
		{Index: 0, Rect: Rect{X: 0, Y: 0, W: 50, H: 100}},
		{Index: 1, Rect: Rect{X: 50, Y: 0, W: 50, H: 100}},
	}
	if got := tileAt(tiles, 10, 10); got != 0 {
		t.Errorf("tileAt(10,10) = %d, want 0", got)
	}
	if got := tileAt(tiles, 60, 10); got != 1 {
		t.Errorf("tileAt(60,10) = %d, want 1", got)
	}
	if got := tileAt(tiles, 200, 10); got != -1 {
		t.Errorf("tileAt outside = %d, want -1", got)
	}
}

func TestTrim(t *testing.T) {
	if got := trim("abcdef", 10); got != "abcdef" {
		t.Errorf("trim short = %q", got)
	}
	if got := trim("abcdef", 4); len([]rune(got)) != 4 {
		t.Errorf("trim long = %q", got)
	}
	if got := trim("abcdef", 0); got != "" {
		t.Errorf("trim zero = %q", got)
	}
}

func TestAnalyzeChosenSelection(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "analyze"
	root := fakeIndex()
	a.analyze.root = root
	a.analyze.current = root
	a.analyze.largest = scan.LargestFiles(root, 20)
	a.analyze.chosen[`C:\Users\ada\big\one.bin`] = true
	a.analyze.view = 1

	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "1 items selected") {
		t.Errorf("selection summary missing:\n%s", texts)
	}
	if !strings.Contains(texts, "Move to Recycle Bin") {
		t.Errorf("trash action missing:\n%s", texts)
	}
	if paths := a.chosenPaths(); len(paths) != 1 || paths[0] != `C:\Users\ada\big\one.bin` {
		t.Errorf("chosenPaths = %v", paths)
	}
}

func TestSplitTrashableRefusesOutsideTheGuard(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "cache", "big.bin")
	outside := filepath.Join(t.TempDir(), "Downloads", "movie.mkv")
	guard := safety.ForRoots(root)

	ok, refused, reason := splitTrashable(guard, []string{inside, outside})
	if len(ok) != 1 || ok[0] != inside {
		t.Fatalf("ok = %v, want only %s", ok, inside)
	}
	if refused != 1 {
		t.Fatalf("refused = %d, want 1", refused)
	}
	if reason != string(safety.ReasonOutsideRoot) {
		t.Fatalf("reason = %q, want the guard's short reason", reason)
	}
}

func TestTrashPathsExplainsARefusalWithoutADialog(t *testing.T) {
	a := newTestApp(t)
	a.Guard = safety.ForRoots(t.TempDir())
	a.trashPaths([]string{filepath.Join(t.TempDir(), "movie.mkv")})
	if !strings.Contains(a.analyze.trashNote, "not app data or a cache") {
		t.Fatalf("trashNote = %q, want the refusal explained", a.analyze.trashNote)
	}
}

func TestTrashOutcomeCountsRefusedAndFailed(t *testing.T) {
	results := []delete.Result{
		{Path: "a"},
		{Path: "b", Err: "the file is in use"},
	}
	if got := trashOutcome(results[:1], 0, ""); got != "" {
		t.Fatalf("all moved, but outcome = %q", got)
	}
	got := trashOutcome(results, 1, "outside allowed folders")
	if !strings.Contains(got, "Not moved to the Recycle Bin: 2 items") || !strings.Contains(got, "outside allowed folders") {
		t.Fatalf("outcome = %q", got)
	}
	got = trashOutcome(results, 0, "")
	if !strings.Contains(got, "Not moved to the Recycle Bin: 1 items") || !strings.Contains(got, "in use") {
		t.Fatalf("outcome = %q", got)
	}
}
