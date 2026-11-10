package locales

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// TestCatalogsMatch checks that every language says every message and keeps
// the placeholders of the source language: a missing key would show English,
// and a renamed placeholder would fail at run time.
func TestCatalogsMatch(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the package directory")
	}
	dir := filepath.Dir(file)
	en := readCatalog(t, filepath.Join(dir, "en.json"))
	vi := readCatalog(t, filepath.Join(dir, "vi.json"))

	for key, source := range en {
		translated, ok := vi[key]
		if !ok {
			t.Errorf("%s is missing from vi.json", key)
			continue
		}
		if got, want := placeholders(translated), placeholders(source); got != want {
			if len(want) == 0 && len(got) == 0 {
				continue
			}
			t.Errorf("%s placeholders = %q, want %q", key, got, want)
		}
	}
	for key := range vi {
		if _, ok := en[key]; !ok {
			t.Errorf("%s is in vi.json but not en.json", key)
		}
	}
}

func readCatalog(t *testing.T, path string) map[string]string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var all map[string]string
	if err := json.Unmarshal(body, &all); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	delete(all, "@@locale")
	for key := range all {
		if len(key) > 0 && key[0] == '@' {
			delete(all, key)
		}
	}
	return all
}

// placeholder matches both {name} and ICU's typed {name, number, integer}.
var placeholder = regexp.MustCompile(`\{([a-zA-Z_]+)(?:,[^}]*)?\}`)

// placeholders returns the argument names in order of appearance. The type of
// an ICU argument may be translated; its name may not.
func placeholders(message string) string {
	matches := placeholder.FindAllStringSubmatch(message, -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match[1])
	}
	return join(names)
}

func join(parts []string) string {
	out := ""
	for _, part := range parts {
		out += part
	}
	return out
}

// TestPlaceholdersSeeICUTypes guards the parity check itself: a typed argument
// must be compared by name, so a translated type cannot hide a renamed name.
func TestPlaceholdersSeeICUTypes(t *testing.T) {
	if got := placeholders("Xoá {count, number, integer} mục?"); got != "count" {
		t.Errorf("typed placeholder = %q", got)
	}
	if got := placeholders("{size} sẽ vào Recycle Bin"); got != "size" {
		t.Errorf("bare placeholder = %q", got)
	}
	if got := placeholders("{count, number, integer}"); got != placeholders("{count, number}") {
		t.Errorf("types changed the argument name")
	}
	if got := placeholders("{count, number, integer}"); got == placeholders("{total, number, integer}") {
		t.Errorf("different names compared equal")
	}
}
