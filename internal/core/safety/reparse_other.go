//go:build !windows

package safety

import (
	"os"
	"path/filepath"
	"strings"
)

// reparseAncestor walks from root down to path and returns the first
// component that is a symbolic link, if any. The Windows build also catches
// junctions and cloud placeholders.
func reparseAncestor(root, path string) (string, bool) {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if !within(root, path) {
		return "", false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	current := root
	if isReparse(root) {
		return root, true
	}
	if rel == "." || rel == "" {
		return "", false
	}
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		if isReparse(current) {
			return current, true
		}
	}
	return "", false
}

func isReparse(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
