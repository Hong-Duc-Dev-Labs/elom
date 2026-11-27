// Package safety decides whether a path may be listed for cleanup and, more
// importantly, whether it may be deleted. Every deleting code path calls
// Check again at the moment of deletion: a path approved by a scan is not a
// trusted input.
package safety

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Reason says why a path was refused. It is part of the app's contract with
// the user: the review screen shows it next to the item.
type Reason string

const (
	ReasonOutsideRoot Reason = "outside allowed folders"
	ReasonProtected   Reason = "Windows or program files"
	ReasonUserData    Reason = "your personal files"
	ReasonReparse     Reason = "a link or junction"
	ReasonUNC         Reason = "a network path"
	ReasonADS         Reason = "an alternate data stream"
	ReasonInvalid      Reason = "not a usable path"
	ReasonNotArtifact  Reason = "not a recognized build artifact"
	ReasonNotInstaller Reason = "not an installer file in Downloads"
)

// Refusal is the error Check returns for a path that must not be touched.
type Refusal struct {
	Path   string
	Reason Reason
}

func (r *Refusal) Error() string {
	return fmt.Sprintf("refusing %s: %s", r.Path, r.Reason)
}

// IsRefusal reports whether err is a Refusal with the given reason.
func IsRefusal(err error, reason Reason) bool {
	var r *Refusal
	return errors.As(err, &r) && r.Reason == reason
}

// Guard holds the folders cleanup may work in and the places it may never
// touch. Build it once with New; it is read-only afterwards.
type Guard struct {
	allow      []string
	protected  []string
	userData   []string
	home       string
	extraCheck func(root, abs string) error
}

// New builds the guard from the current user's environment. Paths come from
// the OS, never from the UI.
func New() *Guard {
	home := env("USERPROFILE", func() string { h, _ := os.UserHomeDir(); return h })
	local := env("LOCALAPPDATA", func() string { return filepath.Join(home, "AppData", "Local") })
	roaming := env("APPDATA", func() string { return filepath.Join(home, "AppData", "Roaming") })
	programData := env("ProgramData", func() string { return `C:\ProgramData` })
	systemRoot := env("SystemRoot", func() string { return `C:\Windows` })

	g := &Guard{
		home: home,
		allow: compact(
			local,
			roaming,
			filepath.Join(local, "Temp"),
			filepath.Join(home, ".npm"),
			filepath.Join(home, ".cargo"),
			filepath.Join(home, ".gradle"),
			filepath.Join(home, ".m2"),
			filepath.Join(home, ".cache"),
			filepath.Join(home, "go", "pkg", "mod"),
			filepath.Join(home, ".bun", "install", "cache"),
			filepath.Join(home, ".rustup", "downloads"),
			filepath.Join(home, ".aws", "cli", "cache"),
			filepath.Join(home, ".azure", "logs"),
			filepath.Join(home, ".kube", "cache"),
		),
		protected: compact(
			systemRoot,
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			os.Getenv("ProgramW6432"),
			filepath.Join(programData, "Microsoft"),
			filepath.Join(programData, "Package Cache"),
		),
		userData: compact(
			filepath.Join(home, "Desktop"),
			filepath.Join(home, "Documents"),
			filepath.Join(home, "Downloads"),
			filepath.Join(home, "Pictures"),
			filepath.Join(home, "Videos"),
			filepath.Join(home, "Music"),
			filepath.Join(home, "Saved Games"),
		),
	}
	g.userData = append(g.userData, oneDriveFolders(home)...)
	return g
}

// ForRoots returns a guard that allows exactly the given roots, for tests and
// for a command line tool that takes its folders as arguments.
func ForRoots(allow ...string) *Guard {
	return &Guard{allow: compact(allow...)}
}

// projectArtifacts is the closed set of build/dependency directory names that
// ForProjectRoots may approve for deletion.
var projectArtifacts = map[string]bool{
	"node_modules":  true,
	"target":        true,
	".venv":         true,
	"venv":          true,
	"__pycache__":   true,
	".pytest_cache": true,
	".mypy_cache":   true,
	".ruff_cache":   true,
	".next":         true,
	".nuxt":         true,
	".turbo":        true,
	".parcel-cache": true,
	".vite":         true,
	"bin":           true,
	"obj":           true,
	".gradle":       true,
	"dist":          true,
	"build":         true,
	"out":           true,
}

// projectMarkers are exact filenames that identify a developer project root.
var projectMarkers = []string{
	"package.json",
	"cargo.toml",
	"go.mod",
	"pyproject.toml",
	"requirements.txt",
	"setup.py",
	"build.gradle",
	"build.gradle.kts",
	"pom.xml",
	"composer.json",
	"cmakelists.txt",
	"pubspec.yaml",
	"mix.exs",
	"gemfile",
	"tsconfig.json",
}

// projectMarkerExts are file extensions (e.g. .NET project files) that
// identify a developer project root.
var projectMarkerExts = []string{
	".csproj",
	".fsproj",
	".vbproj",
	".sln",
}

// IsArtifactName reports whether name is in the closed set of developer build
// and dependency artifact directories.
func IsArtifactName(name string) bool {
	return projectArtifacts[strings.ToLower(strings.TrimSpace(name))]
}

// HasProjectMarker reports whether dir directly contains a recognized project
// manifest or solution file (such as package.json, Cargo.toml, go.mod,
// pyproject.toml, *.csproj, *.sln, build.gradle, pom.xml).
func HasProjectMarker(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		for _, m := range projectMarkers {
			if lower == m {
				return true
			}
		}
		for _, ext := range projectMarkerExts {
			if strings.HasSuffix(lower, ext) && len(lower) > len(ext) {
				return true
			}
		}
	}
	return false
}

// IsReparse reports whether path exists and is a reparse point (junction,
// symbolic link, or cloud placeholder on Windows; symlink elsewhere).
func IsReparse(path string) bool {
	return isReparse(path)
}

// ForProjectRoots returns a specialized guard for Developer Project Purge.
// Beyond the standard UNC, ADS, volume root, OS protected folder, allow-root
// and reparse-point checks, it ONLY approves a path when:
//   - it is strictly inside one of the given project roots (never a root itself),
//   - its basename is in the closed build/dependency artifact list, and
//   - its parent directory contains a valid project marker file.
func ForProjectRoots(roots ...string) *Guard {
	home := env("USERPROFILE", func() string { h, _ := os.UserHomeDir(); return h })
	programData := env("ProgramData", func() string { return `C:\ProgramData` })
	systemRoot := env("SystemRoot", func() string { return `C:\Windows` })
	return &Guard{
		home:  home,
		allow: compact(roots...),
		protected: compact(
			systemRoot,
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			os.Getenv("ProgramW6432"),
			filepath.Join(programData, "Microsoft"),
			filepath.Join(programData, "Package Cache"),
		),
		extraCheck: func(root, abs string) error {
			if normalize(abs) == normalize(root) {
				return &Refusal{Path: abs, Reason: ReasonNotArtifact}
			}
			if !IsArtifactName(filepath.Base(abs)) {
				return &Refusal{Path: abs, Reason: ReasonNotArtifact}
			}
			if !HasProjectMarker(filepath.Dir(abs)) {
				return &Refusal{Path: abs, Reason: ReasonNotArtifact}
			}
			return nil
		},
	}
}

var installerExts = map[string]bool{
	".msi":  true,
	".msix": true,
	".iso":  true,
	".exe":  true,
}

// IsInstallerExt reports whether name has a recognized Windows installer
// extension (.msi, .msix, .iso, .exe).
func IsInstallerExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(name)))
	return installerExts[ext]
}

// ForInstallerDownloads returns a dedicated guard for Old Installer Cleanup.
// It ONLY approves single regular files (never directories, never symlinks or
// reparse points) strictly inside downloadsDir with an allowed installer
// extension (.msi, .msix, .iso, .exe).
func ForInstallerDownloads(downloadsDir string) *Guard {
	home := env("USERPROFILE", func() string { h, _ := os.UserHomeDir(); return h })
	programData := env("ProgramData", func() string { return `C:\ProgramData` })
	systemRoot := env("SystemRoot", func() string { return `C:\Windows` })
	g := &Guard{
		home:  home,
		allow: compact(downloadsDir),
		protected: compact(
			systemRoot,
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			os.Getenv("ProgramW6432"),
			filepath.Join(programData, "Microsoft"),
			filepath.Join(programData, "Package Cache"),
		),
		userData: compact(
			filepath.Join(home, "Desktop"),
			filepath.Join(home, "Documents"),
			filepath.Join(home, "Pictures"),
			filepath.Join(home, "Videos"),
			filepath.Join(home, "Music"),
			filepath.Join(home, "Saved Games"),
		),
		extraCheck: func(root, abs string) error {
			if normalize(abs) == normalize(root) {
				return &Refusal{Path: abs, Reason: ReasonNotInstaller}
			}
			if !IsInstallerExt(filepath.Base(abs)) {
				return &Refusal{Path: abs, Reason: ReasonNotInstaller}
			}
			info, err := os.Lstat(abs)
			if err != nil || info.IsDir() || !info.Mode().IsRegular() {
				return &Refusal{Path: abs, Reason: ReasonNotInstaller}
			}
			return nil
		},
	}
	g.userData = append(g.userData, oneDriveFolders(home)...)
	return g
}

// Check returns nil when path may be deleted, and a *Refusal otherwise. It
// cleans the path, resolves it against the working directory, refuses
// network paths and alternate data streams, then checks the protected
// folders, the personal folders, the allowed roots and finally every
// ancestor's reparse point. A path that does not exist yet has no reparse
// ancestors to check, but is still refused outside the allowed roots.
func (g *Guard) Check(path string) error {
	if strings.TrimSpace(path) == "" {
		return &Refusal{Path: path, Reason: ReasonInvalid}
	}
	if isUNC(path) {
		return &Refusal{Path: path, Reason: ReasonUNC}
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return &Refusal{Path: path, Reason: ReasonInvalid}
	}
	if hasStream(abs) {
		return &Refusal{Path: abs, Reason: ReasonADS}
	}
	if isVolumeRoot(abs) {
		return &Refusal{Path: abs, Reason: ReasonProtected}
	}
	if g.home != "" && normalize(g.home) == normalize(abs) {
		return &Refusal{Path: abs, Reason: ReasonUserData}
	}
	if withinAny(g.protected, abs) {
		return &Refusal{Path: abs, Reason: ReasonProtected}
	}
	if withinAny(g.userData, abs) {
		return &Refusal{Path: abs, Reason: ReasonUserData}
	}
	root, ok := rootOf(g.allow, abs)
	if !ok {
		return &Refusal{Path: abs, Reason: ReasonOutsideRoot}
	}
	if link, ok := reparseAncestor(root, abs); ok {
		return &Refusal{Path: link, Reason: ReasonReparse}
	}
	if g.extraCheck != nil {
		if err := g.extraCheck(root, abs); err != nil {
			return err
		}
	}
	return nil
}

// AllowRoots returns the folders cleanup may work in, cleaned and
// deduplicated. Callers may list them in the UI; they must still call Check
// for every path.
func (g *Guard) AllowRoots() []string { return append([]string(nil), g.allow...) }

// WithinAllowRoot reports whether path is inside an allowed folder, without
// the reparse-point check. Used by the scan to decide what to list.
func (g *Guard) WithinAllowRoot(path string) bool {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}
	_, ok := rootOf(g.allow, abs)
	return ok
}

// isVolumeRoot reports a drive root such as C:\, whose subtree is the whole
// disk: containment checks against it would match every path.
func isVolumeRoot(abs string) bool {
	vol := filepath.VolumeName(abs)
	return vol != "" && (abs == vol || abs == vol+`\`)
}

func env(key string, fallback func() string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback()
}

// oneDriveFolders lists the profile's OneDrive folders, which are personal
// data whatever their localized or tenant suffix. It reads the directory
// once; a folder it cannot list simply adds nothing.
func oneDriveFolders(home string) []string {
	entries, err := os.ReadDir(home)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), "onedrive") {
			out = append(out, filepath.Join(home, e.Name()))
		}
	}
	return out
}

func compact(paths ...string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		p = filepath.Clean(p)
		key := strings.ToLower(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}

func isUNC(path string) bool {
	return strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//")
}

// hasStream reports an alternate data stream (name:stream), allowing the
// drive colon of C:\.
func hasStream(abs string) bool {
	rest := abs
	if vol := filepath.VolumeName(abs); vol != "" {
		rest = abs[len(vol):]
	}
	return strings.Contains(rest, ":")
}

func normalize(path string) string {
	return strings.ToLower(filepath.Clean(path))
}

// within reports whether path is root itself or inside it.
func within(root, path string) bool {
	root, path = normalize(root), normalize(path)
	if root == path {
		return true
	}
	if !strings.HasSuffix(root, `\`) {
		root += `\`
	}
	return strings.HasPrefix(path, root)
}

func withinAny(roots []string, path string) bool {
	for _, r := range roots {
		if within(r, path) {
			return true
		}
	}
	return false
}

// rootOf returns the allowed folder that contains path, if any.
func rootOf(roots []string, path string) (string, bool) {
	best := ""
	for _, r := range roots {
		if within(r, path) && len(r) > len(best) {
			best = r
		}
	}
	return best, best != ""
}
