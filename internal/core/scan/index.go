package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Node is one directory or file of an index. A directory keeps its children
// sorted by size, largest first; a file has none.
type Node struct {
	Path     string
	Name     string
	Dir      bool
	Bytes    int64
	Files    int
	Modified time.Time
	Err      string
	Depth    int
	Children []*Node
}

// Limits bound an index. Zero values mean the defaults below. MaxDepth
// bounds how deep the walk enters directories: 1 lists the root's entries
// without entering its subdirectories.
type Limits struct {
	MaxDepth   int
	MaxEntries int
}

// Defaults for an index.
const (
	DefaultDepth   = 12
	DefaultEntries = 1_000_000
)

// IndexProgress is how far an index got.
type IndexProgress struct {
	Entries int
	Bytes   int64
	Path    string
}

// ErrLimit stops a very large tree from exhausting memory.
var ErrLimit = errors.New("scan: entry limit reached")

// parallelDepth splits the first levels of a walk across goroutines, where a
// profile keeps its bulk (AppData, caches, project folders). Deeper levels
// stay sequential so the number of goroutines stays bounded.
const parallelDepth = 2

// Index walks root and sums what is inside it. A directory it cannot read is
// recorded with its error and does not stop the walk; cancelling ctx returns
// the partial index with ctx's error.
func Index(ctx context.Context, root string, limits Limits, progress func(IndexProgress)) (*Node, error) {
	if limits.MaxDepth <= 0 {
		limits.MaxDepth = DefaultDepth
	}
	if limits.MaxEntries <= 0 {
		limits.MaxEntries = DefaultEntries
	}
	b := &builder{
		limits:   limits,
		progress: progress,
		workers:  min(runtime.NumCPU(), 8),
	}
	node := b.walk(ctx, root, 0)
	if err := ctx.Err(); err != nil {
		return node, err
	}
	if b.limited.Load() {
		return node, ErrLimit
	}
	return node, nil
}

type builder struct {
	limits   Limits
	progress func(IndexProgress)
	workers  int
	entries  atomic.Int64
	bytes    atomic.Int64
	limited  atomic.Bool
	mu       sync.Mutex
}

func (b *builder) atLimit() bool {
	if b.entries.Load() >= int64(b.limits.MaxEntries) {
		b.limited.Store(true)
		return true
	}
	return false
}

func (b *builder) countEntry(path string, bytes int64) {
	n := b.entries.Add(1)
	total := b.bytes.Add(bytes)
	if b.progress != nil && n%1024 == 0 {
		b.mu.Lock()
		b.progress(IndexProgress{Entries: int(n), Bytes: total, Path: path})
		b.mu.Unlock()
	}
}

func (b *builder) walk(ctx context.Context, path string, depth int) *Node {
	node := &Node{Path: path, Name: filepath.Base(path), Dir: true, Depth: depth}
	if err := ctx.Err(); err != nil {
		node.Err = "cancelled"
		return node
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		node.Err = err.Error()
		return node
	}

	children := make([]*Node, 0, len(entries))
	if depth < parallelDepth && len(entries) > 4 {
		sem := make(chan struct{}, b.workers)
		results := make([]*Node, len(entries))
		var wg sync.WaitGroup
		for i, entry := range entries {
			if b.atLimit() || ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(i int, entry os.DirEntry) {
				defer wg.Done()
				defer func() { <-sem }()
				results[i] = b.child(ctx, path, entry, depth)
			}(i, entry)
		}
		wg.Wait()
		for _, child := range results {
			if child != nil {
				children = append(children, child)
			}
		}
	} else {
		for _, entry := range entries {
			if b.atLimit() || ctx.Err() != nil {
				break
			}
			if child := b.child(ctx, path, entry, depth); child != nil {
				children = append(children, child)
			}
		}
	}

	for _, child := range children {
		node.Bytes += child.Bytes
		node.Files += child.Files
		if child.Modified.After(node.Modified) {
			node.Modified = child.Modified
		}
	}
	sort.SliceStable(children, func(i, j int) bool {
		if children[i].Bytes != children[j].Bytes {
			return children[i].Bytes > children[j].Bytes
		}
		return children[i].Name < children[j].Name
	})
	node.Children = children
	if depth == 0 && b.progress != nil {
		b.progress(IndexProgress{Entries: int(b.entries.Load()), Bytes: b.bytes.Load(), Path: path})
	}
	return node
}

// child indexes one entry: a file is measured from its directory entry (one
// stat), a directory recurses. Links and junctions are never followed.
func (b *builder) child(ctx context.Context, parent string, entry os.DirEntry, depth int) *Node {
	path := filepath.Join(parent, entry.Name())
	if entry.IsDir() {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		b.entries.Add(1)
		if depth+1 >= b.limits.MaxDepth {
			return &Node{Path: path, Name: entry.Name(), Dir: true, Depth: depth + 1, Err: "not entered: depth limit"}
		}
		return b.walk(ctx, path, depth+1)
	}
	info, err := entry.Info()
	if err != nil {
		return nil
	}
	b.countEntry(path, info.Size())
	return &Node{
		Path:     path,
		Name:     entry.Name(),
		Bytes:    info.Size(),
		Files:    1,
		Modified: info.ModTime(),
		Depth:    depth + 1,
	}
}

// LargestFiles returns the k largest files in the subtree, largest first.
func LargestFiles(n *Node, k int) []*Node {
	if n == nil || k <= 0 {
		return nil
	}
	var files []*Node
	collectFiles(n, &files)
	sort.SliceStable(files, func(i, j int) bool { return files[i].Bytes > files[j].Bytes })
	if len(files) > k {
		files = files[:k]
	}
	return files
}

func collectFiles(n *Node, out *[]*Node) {
	if n == nil {
		return
	}
	if !n.Dir {
		*out = append(*out, n)
		return
	}
	for _, c := range n.Children {
		collectFiles(c, out)
	}
}

// Find returns the node at path inside the index, or nil.
func Find(root *Node, path string) *Node {
	if root == nil {
		return nil
	}
	if filepath.Clean(root.Path) == filepath.Clean(path) {
		return root
	}
	for _, c := range root.Children {
		if found := Find(c, path); found != nil {
			return found
		}
	}
	return nil
}
