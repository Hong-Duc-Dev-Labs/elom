package apps

import "testing"

func TestParseUninstall(t *testing.T) {
	app, ok := ParseUninstall(
		`HKLM 64\Google Chrome`,
		map[string]string{
			"DisplayName":      "Google Chrome",
			"DisplayVersion":   "131.0.6778.86",
			"Publisher":        "Google LLC",
			"UninstallString":  `"C:\Program Files\Google\Chrome\Application\131\Installer\setup.exe" --uninstall`,
			"InstallLocation":  `C:\Program Files\Google\Chrome\Application`,
			"DisplayIcon":      `C:\Program Files\Google\Chrome\Application\chrome.exe,0`,
		},
		map[string]uint64{"EstimatedSize": 400_000},
	)
	if !ok {
		t.Fatal("valid entry rejected")
	}
	if app.Name != "Google Chrome" || app.Version != "131.0.6778.86" {
		t.Fatalf("app = %+v", app)
	}
	if app.SizeBytes != 400_000*1024 {
		t.Errorf("size = %d", app.SizeBytes)
	}
	if app.UninstallCommand() == "" {
		t.Error("no uninstall command")
	}
	if app.Kind != KindRegistry {
		t.Errorf("kind = %q", app.Kind)
	}
}

func TestParseUninstallSkipsHiddenEntries(t *testing.T) {
	cases := map[string]map[string]string{
		"no name":         {},
		"system component": {"DisplayName": "VC redist", "SystemComponent": "1"},
		"update":           {"DisplayName": "Update for X", "ParentKeyName": "X"},
		"release type":     {"DisplayName": "KB5034441", "ReleaseType": "Update"},
	}
	for name, vals := range cases {
		if _, ok := ParseUninstall("k", vals, nil); ok {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestQuietUninstallPreferred(t *testing.T) {
	app := App{Uninstall: "loud.exe", QuietUninstall: "quiet.exe /S"}
	if got := app.UninstallCommand(); got != "quiet.exe /S" {
		t.Fatalf("command = %q", got)
	}
	if got := (App{}).UninstallCommand(); got != "" {
		t.Fatalf("empty app command = %q", got)
	}
}

func TestNormalizeName(t *testing.T) {
	cases := map[string]string{
		"Google Chrome":            "google chrome",
		"Microsoft Visual C++ 2015-2022 Redistributable (x64)": "microsoft visual c++ redistributable",
		"7-Zip 24.08 (x64)":        "7-zip",
		"Adobe Acrobat (64-bit)":   "adobe acrobat",
		"Node.js":                  "node.js",
	}
	for in, want := range cases {
		if got := NormalizeName(in); got != want {
			t.Errorf("NormalizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLikelyAppFolder(t *testing.T) {
	app := App{Name: "Visual Studio Code", Publisher: "Microsoft Corporation"}
	if got := LikelyAppFolder("Code", app); got != "high" {
		t.Errorf("Code = %q, want high (contained in the app name)", got)
	}
	if got := LikelyAppFolder("Microsoft VS Code", app); got != "medium" {
		t.Errorf("Microsoft VS Code = %q, want medium (publisher)", got)
	}
	if got := LikelyAppFolder("Spotify", app); got != "" {
		t.Errorf("unrelated folder = %q, want none", got)
	}
}

func TestSortApps(t *testing.T) {
	list := []App{{Name: "b", SizeBytes: 10}, {Name: "a", SizeBytes: 30}, {Name: "c", SizeBytes: 10}}
	SortApps(list)
	if list[0].Name != "a" || list[1].Name != "b" || list[2].Name != "c" {
		t.Fatalf("order = %v", list)
	}
}

func TestParseSizeKB(t *testing.T) {
	if got := ParseSizeKB(map[string]string{"EstimatedSize": "2048"}, nil); got != 2048 {
		t.Errorf("string size = %d", got)
	}
	if got := ParseSizeKB(nil, map[string]uint64{"EstimatedSize": 5}); got != 5 {
		t.Errorf("integer size = %d", got)
	}
	if got := ParseSizeKB(nil, nil); got != 0 {
		t.Errorf("missing size = %d", got)
	}
}
