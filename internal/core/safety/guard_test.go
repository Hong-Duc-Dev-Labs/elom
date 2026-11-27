//go:build windows

package safety

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func testGuard() *Guard {
	return &Guard{
		home: `C:\Users\ada`,
		allow: []string{
			`C:\Users\ada\AppData\Local`,
			`C:\Users\ada\AppData\Roaming`,
			`C:\Users\ada\AppData\Local\Temp`,
		},
		protected: []string{
			`C:\Windows`,
			`C:\Program Files`,
			`C:\ProgramData\Microsoft`,
		},
		userData: []string{
			`C:\Users\ada\Desktop`,
			`C:\Users\ada\Documents`,
			`C:\Users\ada\Downloads`,
		},
	}
}

func TestCheck(t *testing.T) {
	g := testGuard()
	tests := []struct {
		name   string
		path   string
		reason Reason // "" means allow
	}{
		{"browser cache", `C:\Users\ada\AppData\Local\Google\Chrome\User Data\Default\Cache`, ""},
		{"temp file", `C:\Users\ada\AppData\Local\Temp\elom.tmp`, ""},
		{"roaming settings", `C:\Users\ada\AppData\Roaming\Elom\settings.json`, ""},
		{"volume root", `C:\`, ReasonProtected},
		{"windows", `C:\Windows\System32\drivers\etc\hosts`, ReasonProtected},
		{"program files", `C:\Program Files\App\app.exe`, ReasonProtected},
		{"programdata microsoft", `C:\ProgramData\Microsoft\Windows\Start Menu`, ReasonProtected},
		{"profile root", `C:\Users\ada`, ReasonUserData},
		{"documents", `C:\Users\ada\Documents\taxes.xlsx`, ReasonUserData},
		{"downloads", `C:\Users\ada\Downloads\setup.exe`, ReasonUserData},
		{"traversal to windows", `C:\Users\ada\AppData\Local\..\..\..\..\Windows`, ReasonProtected},
		{"traversal outside", `C:\Users\ada\AppData\Local\..\..\..\..\..\Other`, ReasonOutsideRoot},
		{"unc", `\\server\share\cache`, ReasonUNC},
		{"alternate stream", `C:\Users\ada\AppData\Local\evil:hidden`, ReasonADS},
		{"another drive", `D:\Games\cache`, ReasonOutsideRoot},
		{"empty", "   ", ReasonInvalid},
		{"relative", `cache\tmp`, ReasonOutsideRoot},
		{"case", `c:\users\ADA\appdata\local\Temp\x.tmp`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := g.Check(tc.path)
			if tc.reason == "" {
				if err != nil {
					t.Fatalf("Check(%q) = %v, want allow", tc.path, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Check(%q) allowed, want %s", tc.path, tc.reason)
			}
			if !IsRefusal(err, tc.reason) {
				t.Fatalf("Check(%q) = %v, want reason %s", tc.path, err, tc.reason)
			}
		})
	}
}

func TestCheckRefusesJunction(t *testing.T) {
	if _, err := exec.LookPath("cmd"); err != nil {
		t.Skip("no cmd.exe to create a junction")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Skipf("cannot create a junction: %v: %s", err, out)
	}
	t.Cleanup(func() { os.Remove(link) })

	g := &Guard{allow: []string{dir}}
	if err := g.Check(link); err == nil || !IsRefusal(err, ReasonReparse) {
		t.Fatalf("Check(junction) = %v, want %s", err, ReasonReparse)
	}
	if err := g.Check(filepath.Join(link, "inside")); err == nil || !IsRefusal(err, ReasonReparse) {
		t.Fatalf("Check(path through junction) = %v, want %s", err, ReasonReparse)
	}
	if err := g.Check(filepath.Join(target, "plain")); err != nil {
		t.Fatalf("Check(plain path) = %v, want allow", err)
	}
}

func TestCheckUsesEnvironment(t *testing.T) {
	t.Setenv("USERPROFILE", `C:\Users\ada`)
	t.Setenv("LOCALAPPDATA", `C:\Users\ada\AppData\Local`)
	t.Setenv("APPDATA", `C:\Users\ada\AppData\Roaming`)
	t.Setenv("SystemRoot", `C:\Windows`)
	t.Setenv("SystemDrive", `C:`)
	t.Setenv("ProgramFiles", `C:\Program Files`)
	t.Setenv("ProgramData", `C:\ProgramData`)

	g := New()
	if len(g.AllowRoots()) == 0 {
		t.Fatal("no allow roots")
	}
	if err := g.Check(`C:\Users\ada\AppData\Local\npm-cache\_cacache`); err != nil {
		t.Fatalf("allowed path refused: %v", err)
	}
	for _, allowed := range []string{
		`C:\Users\ada\.bun\install\cache\pkg`,
		`C:\Users\ada\.rustup\downloads\abc`,
		`C:\Users\ada\.aws\cli\cache\token.json`,
		`C:\Users\ada\.azure\logs\cli.log`,
		`C:\Users\ada\.kube\cache\discovery`,
	} {
		if err := g.Check(allowed); err != nil {
			t.Fatalf("narrow cache path %s refused: %v", allowed, err)
		}
	}
	for _, refused := range []string{
		`C:\Users\ada\.aws\credentials`,
		`C:\Users\ada\.kube\config`,
		`C:\Users\ada\.azure\msal_token_cache.json`,
		`C:\Users\ada\.rustup\toolchains`,
	} {
		if err := g.Check(refused); err == nil {
			t.Fatalf("credential/toolchain path %s was allowed", refused)
		}
	}
	if err := g.Check(`C:\Windows\Temp\evil.dll`); err == nil {
		t.Fatal("C:\\Windows\\Temp allowed")
	}
	if err := g.Check(`C:\Users\ada\Documents\a.docx`); err == nil {
		t.Fatal("Documents allowed")
	}
}

func TestForProjectRoots(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "myapp")
	nm := filepath.Join(proj, "node_modules")
	src := filepath.Join(proj, "src")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "package.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Folder without project marker.
	unmarked := filepath.Join(root, "notes", "build")
	if err := os.MkdirAll(unmarked, 0o755); err != nil {
		t.Fatal(err)
	}

	// .NET project with *.csproj marker.
	dotnet := filepath.Join(root, "dotnetapp")
	dotnetBin := filepath.Join(dotnet, "bin")
	if err := os.MkdirAll(dotnetBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dotnet, "App.csproj"), []byte(`<Project/>`), 0o644); err != nil {
		t.Fatal(err)
	}

	g := ForProjectRoots(root)

	if err := g.Check(nm); err != nil {
		t.Errorf("Check(node_modules) = %v, want allow", err)
	}
	if err := g.Check(dotnetBin); err != nil {
		t.Errorf("Check(dotnet bin) = %v, want allow", err)
	}
	if err := g.Check(root); !IsRefusal(err, ReasonNotArtifact) {
		t.Errorf("Check(root) = %v, want ReasonNotArtifact", err)
	}
	if err := g.Check(src); !IsRefusal(err, ReasonNotArtifact) {
		t.Errorf("Check(src) = %v, want ReasonNotArtifact", err)
	}
	if err := g.Check(unmarked); !IsRefusal(err, ReasonNotArtifact) {
		t.Errorf("Check(unmarked build) = %v, want ReasonNotArtifact", err)
	}
	if err := g.Check(filepath.Join(t.TempDir(), "node_modules")); !IsRefusal(err, ReasonOutsideRoot) {
		t.Errorf("Check(outside root) = %v, want ReasonOutsideRoot", err)
	}
}

func TestForInstallerDownloads(t *testing.T) {
	downloads := t.TempDir()
	msi := filepath.Join(downloads, "setup.msi")
	doc := filepath.Join(downloads, "notes.txt")
	subDir := filepath.Join(downloads, "folder.msi")
	if err := os.WriteFile(msi, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doc, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "other.msi")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := ForInstallerDownloads(downloads)
	if err := g.Check(msi); err != nil {
		t.Errorf("Check(setup.msi) = %v, want allow", err)
	}
	if err := g.Check(downloads); !IsRefusal(err, ReasonNotInstaller) {
		t.Errorf("Check(downloads root) = %v, want ReasonNotInstaller", err)
	}
	if err := g.Check(doc); !IsRefusal(err, ReasonNotInstaller) {
		t.Errorf("Check(notes.txt) = %v, want ReasonNotInstaller", err)
	}
	if err := g.Check(subDir); !IsRefusal(err, ReasonNotInstaller) {
		t.Errorf("Check(directory named folder.msi) = %v, want ReasonNotInstaller", err)
	}
	if err := g.Check(outside); !IsRefusal(err, ReasonOutsideRoot) {
		t.Errorf("Check(outside.msi) = %v, want ReasonOutsideRoot", err)
	}
}
