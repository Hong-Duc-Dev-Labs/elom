package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinCatalogIsValid(t *testing.T) {
	catalog := Builtin()
	if len(catalog) < 25 {
		t.Fatalf("catalog has %d rules, want at least 25", len(catalog))
	}
	for _, r := range catalog {
		if r.Note == "" {
			t.Errorf("%s has no note explaining the effect", r.ID)
		}
		for _, p := range r.Paths {
			if strings.HasPrefix(p, `C:\`) || strings.HasPrefix(p, "C:/") {
				t.Errorf("%s hard-codes a path: %s", r.ID, p)
			}
			if strings.Contains(p, "..") {
				t.Errorf("%s escapes its parent: %s", r.ID, p)
			}
		}
	}
}

func TestDecodeStrict(t *testing.T) {
	_, err := Decode(strings.NewReader(`[{"id":"x","category":"logs","paths":["p"],"typo":1}]`))
	if err == nil {
		t.Fatal("Decode accepted an unknown field")
	}
}

func TestValidate(t *testing.T) {
	cases := map[string][]Rule{
		"empty id":    {{Category: "logs", Paths: []string{"p"}}},
		"duplicate":   {{ID: "a", Category: "logs", Paths: []string{"p"}}, {ID: "A", Category: "logs", Paths: []string{"p"}}},
		"category":    {{ID: "a", Category: "nope", Paths: []string{"p"}}},
		"no paths":    {{ID: "a", Category: "logs"}},
		"negative age": {{ID: "a", Category: "logs", Paths: []string{"p"}, MinAgeDays: -1}},
	}
	for name, rules := range cases {
		if err := Validate(rules); err == nil {
			t.Errorf("%s: Validate accepted the catalog", name)
		}
	}
	if err := Validate([]Rule{{ID: "a", Category: "logs", Paths: []string{"p"}}}); err != nil {
		t.Fatalf("valid catalog rejected: %v", err)
	}
}

func TestExpandEnvAndGlob(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ELOM_TEST_ROOT", root)
	for _, name := range []string{"One", "two", "other"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Expand([]string{`%ELOM_TEST_ROOT%\*`})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("Expand = %v, want 3 entries", got)
	}
	got, err = Expand([]string{`%ELOM_TEST_ROOT%\t*`})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "two" {
		t.Fatalf("Expand(t*) = %v, want only two", got)
	}
}

func TestExpandErrors(t *testing.T) {
	if _, err := Expand([]string{`%ELOM_NOT_SET%\x`}); err == nil {
		t.Fatal("Expand accepted an unset variable")
	}
	if _, err := Expand([]string{`%oops`}); err == nil {
		t.Fatal("Expand accepted an unterminated %")
	}
}

func TestExpandMissingPath(t *testing.T) {
	got, err := Expand([]string{`%TEMP%\elom-does-not-exist-42\*`})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Expand = %v, want nothing", got)
	}
}
