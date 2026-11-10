// Package apps lists what is installed on Windows and what an uninstall
// leaves behind. The registry reader is a thin Windows adapter; the parsing
// and the leftover heuristics are pure functions with tests.
package apps

import (
	"errors"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ErrNoUninstaller means the app registered no way to uninstall it.
var ErrNoUninstaller = errors.New("apps: the app has no uninstaller")

// Kind says where an app's uninstall information came from.
type Kind string

const (
	KindRegistry Kind = "registry"
	KindAppx     Kind = "appx"
)

// App is one installed program.
type App struct {
	ID     string // the uninstall key path, or the packaged app's full name
	Name   string
	Version string
	Publisher string
	InstallLocation string
	Uninstall       string
	QuietUninstall  string
	SizeBytes       int64
	Icon            string
	Kind            Kind
}

// ParseUninstall turns one uninstall registry key's values into an App. It
// reports false for entries that are not user-visible programs: updates,
// components and keys without a display name, as Settings › Apps hides them.
func ParseUninstall(keyPath string, vals map[string]string, ints map[string]uint64) (App, bool) {
	name := strings.TrimSpace(vals["DisplayName"])
	if name == "" {
		return App{}, false
	}
	if component := strings.TrimSpace(vals["SystemComponent"]); component == "1" {
		return App{}, false
	}
	if parent := strings.TrimSpace(vals["ParentKeyName"]); parent != "" {
		return App{}, false
	}
	if update := strings.TrimSpace(vals["ReleaseType"]); update != "" &&
		!strings.EqualFold(update, "Product") &&
		!strings.EqualFold(update, "Application") {
		return App{}, false
	}
	size := int64(0)
	if kb, ok := ints["EstimatedSize"]; ok {
		size = int64(kb) * 1024
	}
	return App{
		ID:              keyPath,
		Name:            name,
		Version:         strings.TrimSpace(vals["DisplayVersion"]),
		Publisher:       strings.TrimSpace(vals["Publisher"]),
		InstallLocation: strings.TrimSpace(vals["InstallLocation"]),
		Uninstall:       strings.TrimSpace(vals["UninstallString"]),
		QuietUninstall:  strings.TrimSpace(vals["QuietUninstallString"]),
		SizeBytes:       size,
		Icon:            strings.TrimSpace(vals["DisplayIcon"]),
		Kind:            KindRegistry,
	}, true
}

// SortApps orders by size, largest first, then by name.
func SortApps(list []App) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].SizeBytes != list[j].SizeBytes {
			return list[i].SizeBytes > list[j].SizeBytes
		}
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
}

// UninstallCommand returns the command to run, preferring the quiet one. It
// never invents a command: an app without one is reported as not uninstallable
// by the caller.
func (a App) UninstallCommand() string {
	if a.QuietUninstall != "" {
		return a.QuietUninstall
	}
	return a.Uninstall
}

// NormalizeName prepares an app name for leftover matching: lower case,
// without version numbers, publisher suffixes and punctuation.
func NormalizeName(name string) string {
	lower := strings.ToLower(name)
	fields := strings.Fields(lower)
	kept := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.Trim(field, ".,()[]{}·-–—:;'\"")
		if trimmed == "" {
			continue
		}
		if strings.ContainsAny(trimmed, "0123456789") && looksLikeVersion(trimmed) {
			continue
		}
		switch trimmed {
		case "inc", "ltd", "llc", "gmbh", "corp", "corporation", "the", "for", "x64", "x86", "64-bit", "32-bit":
			continue
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, " ")
}

// looksLikeVersion reports a token that is only digits and dots, such as
// 1.2.3 or 2024.
func looksLikeVersion(token string) bool {
	if token == "" {
		return false
	}
	digits := 0
	for _, r := range token {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '.' || r == '_' || r == '-' || r == 'v':
		default:
			return false
		}
	}
	return digits > 0
}

// LikelyAppFolder reports how strongly a directory name looks like it belongs
// to the app: "high" when the normalized names match or contain each other,
// "medium" for a publisher match, "" otherwise.
func LikelyAppFolder(dirName string, app App) string {
	folder := NormalizeName(dirName)
	if folder == "" {
		return ""
	}
	name := NormalizeName(app.Name)
	if name != "" && (folder == name || strings.Contains(folder, name) || strings.Contains(name, folder)) {
		return "high"
	}
	publisher := NormalizeName(app.Publisher)
	if publisher != "" && len(publisher) > 3 && strings.Contains(folder, publisher) {
		return "medium"
	}
	return ""
}

// BaseName is the folder name of a path, for display and matching.
func BaseName(path string) string { return filepath.Base(path) }

// ParseSizeKB reads a registry size value that some vendors store as a string.
func ParseSizeKB(vals map[string]string, ints map[string]uint64) int64 {
	if kb, ok := ints["EstimatedSize"]; ok {
		return int64(kb)
	}
	if text := vals["EstimatedSize"]; text != "" {
		if kb, err := strconv.ParseInt(text, 10, 64); err == nil {
			return kb
		}
	}
	return 0
}
