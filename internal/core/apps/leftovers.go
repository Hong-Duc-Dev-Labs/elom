package apps

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"elom/internal/core/safety"
)

const (
	// DefaultOrphanMinAgeDays is the conservative inactivity threshold before
	// an unmatched AppData folder is offered as orphaned.
	DefaultOrphanMinAgeDays = 60
	// DefaultOrphanMinBytes is the conservative minimum size (10 MB) for an
	// orphaned AppData candidate.
	DefaultOrphanMinBytes = 10 * 1024 * 1024
)

// Leftover is a folder that probably belonged to an uninstalled app.
type Leftover struct {
	Path       string
	Confidence string // "high", "medium", or "low" (orphaned)
	Reason     string
	// Bytes is measured once, when the leftover is found, so views never
	// walk the tree to draw a row.
	Bytes   int64
	AgeDays int
	Checked bool
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
	return FindLeftoversWithSiblings(app, nil, roots, exclude)
}

// FindLeftoversWithSiblings behaves like FindLeftovers while applying the
// sibling-install guard: before proposing a "medium" confidence folder that
// matched only by Publisher, it checks whether another installed app shares
// the same normalized Publisher. If a sibling is still installed, the
// publisher's root folder is skipped (though level-2 app subfolders inside it
// are still checked for "high" confidence matches).
func FindLeftoversWithSiblings(app App, installed []App, roots []string, exclude func(string) bool) []Leftover {
	hasSibling := hasInstalledSibling(app, installed)
	seen := map[string]bool{}
	var out []Leftover
	add := func(dir, parent string) bool {
		if seen[dir] || (exclude != nil && exclude(dir)) {
			return false
		}
		confidence := LikelyAppFolder(filepath.Base(dir), app)
		if confidence == "" && parent != "" {
			confidence = LikelyAppFolder(parent+" "+filepath.Base(dir), app)
		}
		if confidence == "" {
			return false
		}
		if confidence == "medium" && hasSibling {
			return false
		}
		seen[dir] = true
		reason := "name matches the app"
		if confidence == "medium" {
			reason = "name matches the publisher"
		}
		out = append(out, Leftover{Path: dir, Confidence: confidence, Reason: reason})
		return true
	}

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			full := filepath.Join(root, entry.Name())
			if safety.IsReparse(full) {
				continue
			}
			conf := LikelyAppFolder(entry.Name(), app)
			if strings.EqualFold(entry.Name(), app.Name) || conf == "high" {
				add(full, "")
				continue
			}
			if conf == "medium" && !hasSibling {
				add(full, "")
				continue
			}
			// One more level: publisher\app (also reached when a publisher
			// folder was protected by the sibling-install guard).
			children, err := os.ReadDir(full)
			if err != nil {
				continue
			}
			for _, child := range children {
				if child.IsDir() && child.Type()&os.ModeSymlink == 0 {
					childPath := filepath.Join(full, child.Name())
					if safety.IsReparse(childPath) {
						continue
					}
					add(childPath, entry.Name())
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

// hasInstalledSibling reports whether another installed app (different ID and
// name) shares app's normalized Publisher.
func hasInstalledSibling(app App, installed []App) bool {
	pub := NormalizeName(app.Publisher)
	if pub == "" || len(pub) <= 3 {
		return false
	}
	for _, other := range installed {
		if other.ID != "" && app.ID != "" && other.ID == app.ID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(other.Name), strings.TrimSpace(app.Name)) {
			continue
		}
		if NormalizeName(other.Publisher) == pub {
			return true
		}
	}
	return false
}

// orphanProtectedNames excludes Windows system folders, hardware drivers,
// frameworks/runtimes, AI tools, VPN clients, password managers, browsers,
// and cloud sync folders from orphaned app-data suggestions.
var orphanProtectedNames = map[string]bool{
	// System / OS
	"microsoft": true, "windows": true, "windowsapps": true, "packages": true,
	"temp": true, "tmp": true, "programs": true, "common files": true,
	"squirreltemp": true, "d3dscache": true, "elevateddiagnostics": true,
	"crashdumps": true, "connecteddevicesplatform": true, "comms": true,
	"publishers": true, "peerdistrepub": true, "speech": true, "assembly": true,
	"cefsharp": true, "webview2": true, "diagnostics": true, "history": true,
	"iconcache": true, "low": true, "locallow": true, "roaming": true,
	"local": true, "fontcache": true, "identitycrl": true, "inetsrv": true,
	"internet explorer": true, "licensing": true, "networklist": true,
	"oleacc": true, "package cache": true, "systemcertificates": true,
	"tiledatalayer": true, "usoprivate": true, "wer": true, "elom": true,
	// Drivers & Hardware
	"nvidia": true, "nvidia corporation": true, "amd": true, "ati": true,
	"intel": true, "realtek": true, "qualcomm": true, "broadcom": true,
	"logitech": true, "logi": true, "razer": true, "corsair": true,
	"steelseries": true, "asus": true, "dell": true, "hp": true, "lenovo": true,
	"synaptics": true, "elan": true, "wacom": true, "displaylink": true,
	// Frameworks & Dev Runtimes
	"dotnet": true, ".dotnet": true, "nuget": true, "python": true, "pip": true,
	"uv": true, "npm": true, "npm-cache": true, "pnpm": true, "yarn": true,
	"bun": true, "cargo": true, "rustup": true, "go": true, "go-build": true,
	"gradle": true, "maven": true, "java": true, "oracle": true, "openjdk": true,
	"jetbrains": true, "visual studio": true, "vscode": true, "code": true,
	"git": true, "github": true, "docker": true, "wsl": true, "scoop": true,
	"chocolatey": true, "winget": true, "msys": true, "mingw": true,
	"llvm": true, "clang": true, "cmake": true, "flutter": true, "dart": true,
	"android": true, "google": true,
	// AI Tools
	"claude": true, "anthropic": true, "openai": true, "chatgpt": true,
	"cursor": true, "windsurf": true, "ollama": true, "lmstudio": true,
	"lm-studio": true, "jan": true, "huggingface": true, "antigravity": true,
	"gemini": true, "copilot": true, "codex": true, "continue": true,
	"aider": true, "perplexity": true, "poe": true,
	// VPN & Networking
	"wireguard": true, "openvpn": true, "tailscale": true, "zerotier": true,
	"nordvpn": true, "expressvpn": true, "proton": true, "protonvpn": true,
	"mullvad": true, "surfshark": true, "cloudflare": true, "warp": true,
	"cisco": true, "anyconnect": true, "forticlient": true, "globalprotect": true,
	"pulse": true, "ivanti": true, "clash": true, "v2ray": true, "sing-box": true,
	"shadowsocks": true,
	// Password Managers & Security
	"1password": true, "bitwarden": true, "keepass": true, "keepassxc": true,
	"dashlane": true, "lastpass": true, "nordpass": true, "enpass": true,
	"roboform": true, "proton pass": true, "yubico": true, "gnupg": true,
	"gpg": true, "ssh": true, "authy": true, "ente": true, "malwarebytes": true,
	"kaspersky": true, "eset": true, "bitdefender": true, "crowdstrike": true,
	"sentinelone": true,
	// Browsers & Cloud Sync
	"chrome": true, "chromium": true, "edge": true, "firefox": true,
	"mozilla": true, "brave": true, "vivaldi": true, "opera": true, "arc": true,
	"zen": true, "onedrive": true, "dropbox": true, "icloud": true,
	"nextcloud": true, "syncthing": true, "megasync": true, "box": true,
	"pcloud": true,
}

func isOrphanProtected(dirName string) bool {
	lower := strings.ToLower(strings.TrimSpace(dirName))
	if lower == "" {
		return true
	}
	if orphanProtectedNames[lower] || orphanProtectedNames[NormalizeName(lower)] {
		return true
	}
	if strings.HasPrefix(lower, "regid.") || strings.HasPrefix(lower, "microsoft.") || strings.HasPrefix(lower, "windows.") {
		return true
	}
	return false
}

// FindOrphaned scans top-level folders in roots for inactive, large folders
// that do not match any currently installed app and are not on the protected
// system/driver/framework/AI/VPN/password-manager exclusion list. Returned
// items always have Checked=false (review-first).
func FindOrphaned(installed []App, roots []string, minAgeDays int, minBytes int64) []Leftover {
	return FindOrphanedFiltered(installed, roots, minAgeDays, minBytes, time.Now(), nil)
}

// FindOrphanedFiltered is FindOrphaned with an explicit clock and exclusion
// callback (such as safety.Guard.Check) for services and deterministic tests.
func FindOrphanedFiltered(installed []App, roots []string, minAgeDays int, minBytes int64, now time.Time, exclude func(string) bool) []Leftover {
	if minAgeDays <= 0 {
		minAgeDays = DefaultOrphanMinAgeDays
	}
	if minBytes <= 0 {
		minBytes = DefaultOrphanMinBytes
	}
	if now.IsZero() {
		now = time.Now()
	}
	minAge := time.Duration(minAgeDays) * 24 * time.Hour

	seen := map[string]bool{}
	var out []Leftover

	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			name := entry.Name()
			if isOrphanProtected(name) {
				continue
			}
			full := filepath.Join(root, name)
			key := strings.ToLower(filepath.Clean(full))
			if seen[key] || safety.IsReparse(full) {
				continue
			}
			if exclude != nil && exclude(full) {
				continue
			}
			if matchesAnyInstalled(name, full, installed) {
				continue
			}

			size, latest := measureDirAge(full)
			if size < minBytes || latest.IsZero() {
				continue
			}
			age := now.Sub(latest)
			if age < minAge {
				continue
			}
			seen[key] = true
			out = append(out, Leftover{
				Path:       full,
				Confidence: "low",
				Reason:     "no matching installed app",
				Bytes:      size,
				AgeDays:    int(age / (24 * time.Hour)),
				Checked:    false,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		return strings.ToLower(out[i].Path) < strings.ToLower(out[j].Path)
	})
	return out
}

func matchesAnyInstalled(dirName, fullPath string, installed []App) bool {
	normFull := strings.ToLower(filepath.Clean(fullPath))
	for _, app := range installed {
		if LikelyAppFolder(dirName, app) != "" {
			return true
		}
		if loc := strings.TrimSpace(app.InstallLocation); loc != "" {
			normLoc := strings.ToLower(filepath.Clean(loc))
			if normLoc == normFull || strings.EqualFold(filepath.Base(normLoc), dirName) {
				return true
			}
		}
	}
	return false
}

func measureDirAge(dir string) (total int64, latest time.Time) {
	if info, err := os.Stat(dir); err == nil {
		latest = info.ModTime()
	}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || (d.IsDir() && safety.IsReparse(p)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		if !d.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, latest
}
