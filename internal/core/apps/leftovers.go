package apps

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Leftover is a folder that probably belonged to an uninstalled app.
type Leftover struct {
	Path       string
	Confidence string // "high" or "medium"
	Reason     string
	// Bytes is measured once, when the leftover is found, so views never
	// walk the tree to draw a row.
	Bytes int64
}

// LeftoverRoots are the folders an uninstall leaves things in, for the current
// user. Personal folders are deliberately absent.
func LeftoverRoots(home, local, roaming, programData string) []string {
	return []string{
		filepath.Join(local, "Programs"),
		local,
		roaming,
		filepath.Join(programData),
	}
}

// FindLeftovers looks one level inside each root, and one more level for
// publisher folders such as Microsoft\VS Code, for folders that match the app.
// It never looks inside personal folders and never follows a link.
func FindLeftovers(app App, roots []string, exclude func(string) bool) []Leftover {
	seen := map[string]bool{}
	var out []Leftover
	add := func(dir, parent string) {
		if seen[dir] || exclude != nil && exclude(dir) {
			return
		}
		confidence := LikelyAppFolder(filepath.Base(dir), app)
		if confidence == "" && parent != "" {
			confidence = LikelyAppFolder(parent+" "+filepath.Base(dir), app)
		}
		if confidence == "" {
			return
		}
		seen[dir] = true
		reason := "name matches the app"
		if confidence == "medium" {
			reason = "name matches the publisher"
		}
		out = append(out, Leftover{Path: dir, Confidence: confidence, Reason: reason})
	}

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			full := filepath.Join(root, entry.Name())
			if strings.EqualFold(entry.Name(), app.Name) || LikelyAppFolder(entry.Name(), app) != "" {
				add(full, "")
				continue
			}
			// One more level: publisher\app.
			children, err := os.ReadDir(full)
			if err != nil {
				continue
			}
			for _, child := range children {
				if child.IsDir() {
					add(filepath.Join(full, child.Name()), entry.Name())
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence == "high"
		}
		return out[i].Path < out[j].Path
	})
	return out
}
