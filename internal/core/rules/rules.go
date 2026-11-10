// Package rules holds the cleanup catalog: what Elom may offer to remove, as
// data rather than code. A rule expands to paths, the scanner sizes them,
// and the guard has the last word before anything is deleted.
package rules

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Rule is one catalog entry.
type Rule struct {
	ID             string   `json:"id"`
	Category       string   `json:"category"`
	App            string   `json:"app"`
	Paths          []string `json:"paths"`
	MinAgeDays     int      `json:"minAgeDays"`
	Regenerates    bool     `json:"regenerates"`
	DefaultChecked bool     `json:"defaultChecked"`
	RequiresAdmin  bool     `json:"requiresAdmin"`
	Note           string   `json:"note"`
}

// Categories orders the review screen and labels the groups.
var Categories = []string{"browser", "app-cache", "logs", "dev-cache", "system-temp"}

//go:embed data/en.json
var builtinJSON []byte

// Builtin returns the catalog shipped with the app.
func Builtin() []Rule {
	rules, err := Decode(strings.NewReader(string(builtinJSON)))
	if err != nil {
		// The catalog is embedded at build time and covered by a test.
		panic("rules: builtin catalog: " + err.Error())
	}
	return rules
}

// Decode reads a catalog strictly: an unknown field is an error, so a typo in
// the catalog fails loudly instead of silently dropping a rule's meaning.
func Decode(r io.Reader) ([]Rule, error) {
	var rules []Rule
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rules); err != nil {
		return nil, fmt.Errorf("rules: %w", err)
	}
	if err := Validate(rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// Validate reports the first problem in a catalog.
func Validate(rules []Rule) error {
	seen := make(map[string]bool, len(rules))
	for i, r := range rules {
		switch {
		case r.ID == "":
			return fmt.Errorf("rules: rule %d has no id", i)
		case seen[strings.ToLower(r.ID)]:
			return fmt.Errorf("rules: duplicate id %q", r.ID)
		case !slices.Contains(Categories, r.Category):
			return fmt.Errorf("rules: %q has unknown category %q", r.ID, r.Category)
		case len(r.Paths) == 0:
			return fmt.Errorf("rules: %q has no paths", r.ID)
		case r.MinAgeDays < 0:
			return fmt.Errorf("rules: %q has a negative age", r.ID)
		}
		seen[strings.ToLower(r.ID)] = true
	}
	return nil
}
