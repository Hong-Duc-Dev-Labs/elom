// Package winget lists and applies Microsoft Store and community updates
// through winget. Its output is a localized table, so parsing is isolated
// here, tested against fixtures, and reported as unparseable rather than
// guessed.
package winget

import (
	"errors"
	"strings"
)

// Upgrade is one app with an available update.
type Upgrade struct {
	Name      string
	ID        string
	Current   string
	Available string
	Source    string
}

// ErrUnparseable means the output was not a table Elom recognizes, usually
// because winget answered in another language or printed a message.
var ErrUnparseable = errors.New("winget: unrecognized output")

// ParseUpgrade reads the table of `winget upgrade`. It returns an empty list
// without error when winget says there is nothing to update.
func ParseUpgrade(out string) ([]Upgrade, error) {
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	headerIndex, columns := findHeader(lines)
	if headerIndex < 0 {
		if saysNothingToUpdate(out) {
			return nil, nil
		}
		return nil, ErrUnparseable
	}
	var upgrades []Upgrade
	for _, line := range lines[headerIndex+1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isRule(trimmed) || strings.HasPrefix(trimmed, "Upgrades available") ||
			strings.Contains(trimmed, "upgrades available") {
			continue
		}
		fields := sliceColumns(line, columns)
		if strings.TrimSpace(fields["Id"]) == "" {
			continue
		}
		upgrades = append(upgrades, Upgrade{
			Name:      strings.TrimSpace(fields["Name"]),
			ID:        strings.TrimSpace(fields["Id"]),
			Current:   strings.TrimSpace(fields["Version"]),
			Available: strings.TrimSpace(fields["Available"]),
			Source:    strings.TrimSpace(fields["Source"]),
		})
	}
	return upgrades, nil
}

// findHeader locates the column header and the offset of each column in it.
func findHeader(lines []string) (int, map[string]int) {
	for i, line := range lines {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "id") || !strings.Contains(lower, "name") {
			continue
		}
		columns := map[string]int{}
		offset := 0
		for _, field := range []string{"Name", "Id", "Version", "Available", "Source"} {
			at := strings.Index(lower[offset:], strings.ToLower(field))
			if at < 0 {
				continue
			}
			columns[field] = offset + at
			offset += at + len(field)
		}
		if _, ok := columns["Available"]; !ok {
			continue
		}
		if _, ok := columns["Id"]; !ok {
			continue
		}
		return i, columns
	}
	return -1, nil
}

// sliceColumns cuts a row at the header's column offsets. Version and
// Available are optional in old winget builds.
func sliceColumns(line string, columns map[string]int) map[string]string {
	order := make([]string, 0, len(columns))
	for name := range columns {
		order = append(order, name)
	}
	// Sort by offset so each field runs to the next column.
	for i := 0; i < len(order); i++ {
		for j := i + 1; j < len(order); j++ {
			if columns[order[j]] < columns[order[i]] {
				order[i], order[j] = order[j], order[i]
			}
		}
	}
	out := map[string]string{}
	for i, name := range order {
		start := columns[name]
		if start > len(line) {
			out[name] = ""
			continue
		}
		end := len(line)
		if i+1 < len(order) && columns[order[i+1]] < end {
			end = columns[order[i+1]]
		}
		out[name] = line[start:end]
	}
	return out
}

func isRule(line string) bool {
	for _, r := range line {
		if r != '-' && r != '=' && r != ' ' && r != '\t' {
			return false
		}
	}
	return true
}

func saysNothingToUpdate(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "no installed package found") ||
		strings.Contains(lower, "no applicable upgrade") ||
		strings.Contains(lower, "upgrades available") && strings.Contains(lower, "0 ")
}
