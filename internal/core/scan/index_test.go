package scan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func buildTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile := func(rel string, size int) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("big/a.bin", 1000)
	writeFile("big/b.bin", 500)
	writeFile("small/c.bin", 100)
	writeFile("top.bin", 50)
	return root
}

func TestIndexSumsAndSorts(t *testing.T) {
	root := buildTree(t)
	node, err := Index(context.Background(), root, Limits{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if node.Bytes != 1650 || node.Files != 4 {
		t.Fatalf("root = %d bytes in %d files, want 1650 in 4", node.Bytes, node.Files)
	}
	if len(node.Children) == 0 || node.Children[0].Name != "big" {
		t.Fatalf("children = %+v, want big first", node.Children)
	}
	if node.Children[0].Bytes != 1500 {
		t.Errorf("big = %d bytes, want 1500", node.Children[0].Bytes)
	}
	// Children of big: a.bin (1000) before b.bin (500).
	big := node.Children[0]
	if len(big.Children) != 2 || big.Children[0].Bytes != 1000 {
		t.Errorf("big children = %+v", big.Children)
	}
	if node.Children[0].Dir != true || big.Children[0].Dir != false {
		t.Error("Dir flag is wrong")
	}
}

func TestIndexLargestFiles(t *testing.T) {
	root := buildTree(t)
	node, err := Index(context.Background(), root, Limits{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := LargestFiles(node, 2)
	if len(got) != 2 || got[0].Bytes != 1000 || got[1].Bytes != 500 {
		t.Fatalf("largest = %+v", got)
	}
	if got[0].Dir {
		t.Error("a file is marked as a directory")
	}
}

func TestIndexDepthLimit(t *testing.T) {
	root := buildTree(t)
	node, err := Index(context.Background(), root, Limits{MaxDepth: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if node.Bytes != 50 {
		t.Fatalf("depth 1 index = %d bytes, want only the top file (50)", node.Bytes)
	}
	if !node.Partial {
		t.Fatal("root should be marked Partial when depth limit is hit")
	}
	hasDepth := false
	for _, r := range node.PartialReasons {
		if r == PartialDepthLimit {
			hasDepth = true
		}
	}
	if !hasDepth {
		t.Fatalf("PartialReasons = %v, want %q", node.PartialReasons, PartialDepthLimit)
	}
}

func TestIndexEntryLimit(t *testing.T) {
	root := buildTree(t)
	node, err := Index(context.Background(), root, Limits{MaxEntries: 2}, nil)
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("Index = %v, want ErrLimit", err)
	}
	if node == nil || !node.Partial {
		t.Fatalf("node should be Partial on entry limit: %+v", node)
	}
	hasEntry := false
	for _, r := range node.PartialReasons {
		if r == PartialEntryLimit {
			hasEntry = true
		}
	}
	if !hasEntry {
		t.Fatalf("PartialReasons = %v, want %q", node.PartialReasons, PartialEntryLimit)
	}
}

func TestIndexCleanableCacheDirTagAndBuildDirs(t *testing.T) {
	root := t.TempDir()
	tagged := filepath.Join(root, "custom-cache")
	if err := os.MkdirAll(tagged, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tagged, "CACHEDIR.TAG"), []byte("Signature: 8a477f597d28d172789f06886806bc55\n# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tagged, "blob.bin"), make([]byte, 512), 0o644); err != nil {
		t.Fatal(err)
	}

	bogusTag := filepath.Join(root, "not-a-cache")
	if err := os.MkdirAll(bogusTag, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bogusTag, "CACHEDIR.TAG"), []byte("wrong header"), 0o644); err != nil {
		t.Fatal(err)
	}

	nm := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nm, "a.js"), make([]byte, 256), 0o644); err != nil {
		t.Fatal(err)
	}

	node, err := Index(context.Background(), root, Limits{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := Find(node, tagged); got == nil || !got.Cleanable {
		t.Errorf("CACHEDIR.TAG folder should be Cleanable: %+v", got)
	}
	if got := Find(node, bogusTag); got == nil || got.Cleanable {
		t.Errorf("invalid CACHEDIR.TAG folder should not be Cleanable: %+v", got)
	}
	if got := Find(node, nm); got == nil || !got.Cleanable {
		t.Errorf("node_modules should be Cleanable: %+v", got)
	}
}

func TestIndexTimeoutMarksPartial(t *testing.T) {
	root := buildTree(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	node, err := Index(ctx, root, Limits{}, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if node == nil || !node.Partial {
		t.Fatalf("timed out node should be Partial: %+v", node)
	}
	hasTimeout := false
	for _, r := range node.PartialReasons {
		if r == PartialTimeout {
			hasTimeout = true
		}
	}
	if !hasTimeout {
		t.Fatalf("PartialReasons = %v, want %q", node.PartialReasons, PartialTimeout)
	}
}

func TestIndexUnreadableDirectory(t *testing.T) {
	root := buildTree(t)
	node, err := Index(context.Background(), root, Limits{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "gone")
	if n := Find(node, missing); n != nil {
		t.Fatalf("Find(%s) = %+v, want nil", missing, n)
	}
	partial, err := Index(context.Background(), missing, Limits{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if partial.Err == "" {
		t.Error("a missing root has no error")
	}
}

func TestIndexCancel(t *testing.T) {
	root := buildTree(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	node, err := Index(ctx, root, Limits{}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Index = %v, want context.Canceled", err)
	}
	if node == nil {
		t.Fatal("cancel returned no partial index")
	}
}

func TestFind(t *testing.T) {
	root := buildTree(t)
	node, err := Index(context.Background(), root, Limits{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "big", "a.bin")
	got := Find(node, want)
	if got == nil || got.Bytes != 1000 {
		t.Fatalf("Find = %+v", got)
	}
}

// TestRealIndex is opt-in: it indexes this PC's user profile, which reads
// hundreds of thousands of entries. Set ELOM_REAL_INDEX=1.
func TestRealIndex(t *testing.T) {
	if os.Getenv("ELOM_REAL_INDEX") == "" {
		t.Skip("set ELOM_REAL_INDEX=1 to index the user profile")
	}
	root := os.Getenv("USERPROFILE")
	limits := Limits{}
	if v := os.Getenv("ELOM_REAL_INDEX_MAX"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		limits.MaxEntries = n
	}
	start := time.Now()
	node, err := Index(context.Background(), root, limits, nil)
	if err != nil && !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	t.Logf("%s: %d entries, %d dirs at top, %s, %s (%v)",
		root, node.Files, len(node.Children), humanBytes(node.Bytes), time.Since(start).Round(time.Millisecond), err)
	if largest := LargestFiles(node, 5); len(largest) > 0 {
		for _, f := range largest {
			t.Logf("  %s %s", humanBytes(f.Bytes), f.Path)
		}
	}
}

func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}
