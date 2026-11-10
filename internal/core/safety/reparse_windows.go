//go:build windows

package safety

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// reparseAncestor walks from root down to path and returns the first
// component that is a reparse point (a junction, a symbolic link or a cloud
// placeholder), if any. Deleting through one would act on a folder outside
// the allowed roots.
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

// isReparse reports whether the path exists and carries the reparse point
// attribute. A missing path is not a reparse point.
func isReparse(path string) bool {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return false
	}
	return attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
