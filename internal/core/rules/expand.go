package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Expand turns a rule's path patterns into concrete paths: environment
// variables first (%LOCALAPPDATA%, %TEMP%, …), then one directory level per
// `*` segment. A pattern that matches nothing yields nothing; it never walks
// upward and never resolves `..`, so a pattern cannot escape its parent.
func Expand(patterns []string) ([]string, error) {
	var out []string
	for _, pattern := range patterns {
		expanded, err := expandEnv(pattern)
		if err != nil {
			return nil, err
		}
		paths, err := expandPath(expanded)
		if err != nil {
			return nil, err
		}
		out = append(out, paths...)
	}
	return out, nil
}

// expandEnv replaces %NAME% with the environment's value. A name that is not
// set is an error: a rule naming a folder the user does not have is skipped
// by the caller, not guessed.
func expandEnv(pattern string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(pattern); {
		if pattern[i] != '%' {
			b.WriteByte(pattern[i])
			i++
			continue
		}
		end := strings.IndexByte(pattern[i+1:], '%')
		if end < 0 {
			return "", fmt.Errorf("rules: unterminated %% in %q", pattern)
		}
		name := pattern[i+1 : i+1+end]
		value := os.Getenv(name)
		if value == "" {
			return "", fmt.Errorf("rules: %%%s%% is not set", name)
		}
		b.WriteString(value)
		i += end + 2
	}
	return b.String(), nil
}

// expandPath splits the path into segments and expands them from the volume
// root (or the working directory for a relative path).
func expandPath(path string) ([]string, error) {
	volume := filepath.VolumeName(path)
	rest := strings.TrimPrefix(path, volume)
	rest = strings.Trim(rest, `\/`)
	if rest == "" {
		return nil, nil
	}
	base := volume + string(os.PathSeparator)
	if volume == "" {
		base = "."
	}
	segments := strings.Split(rest, string(os.PathSeparator))
	return expandSegments(base, segments)
}

// expandSegments expands segments[0] under base and recurses. A segment
// without a `*` is appended; a segment with one lists base and matches each
// entry, case-insensitively, as Windows does.
func expandSegments(base string, segments []string) ([]string, error) {
	if len(segments) == 0 {
		if _, err := os.Lstat(base); err != nil {
			return nil, nil
		}
		return []string{filepath.Clean(base)}, nil
	}
	segment, rest := segments[0], segments[1:]
	if !strings.Contains(segment, "*") {
		return expandSegments(filepath.Join(base, segment), rest)
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, nil
	}
	pattern := strings.ToLower(segment)
	var out []string
	for _, entry := range entries {
		ok, err := filepath.Match(pattern, strings.ToLower(entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("rules: bad pattern %q: %w", segment, err)
		}
		if !ok {
			continue
		}
		more, err := expandSegments(filepath.Join(base, entry.Name()), rest)
		if err != nil {
			return nil, err
		}
		out = append(out, more...)
	}
	return out, nil
}
