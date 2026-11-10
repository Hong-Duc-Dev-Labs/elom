package winget

import (
	"errors"
	"testing"
)

const englishTable = `Name                                Id                                  Version      Available    Source
---------------------------------------------------------------------------------------------------------
Git                                 Git.Git                             2.45.1       2.46.0       winget
Microsoft Visual Studio Code        Microsoft.VisualStudioCode          1.90.0       1.91.2       winget
PowerShell                          Microsoft.PowerShell                7.4.1        7.4.5        winget
3 upgrades available.
`

func TestParseUpgrade(t *testing.T) {
	got, err := ParseUpgrade(englishTable)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d upgrades: %+v", len(got), got)
	}
	first := got[0]
	if first.Name != "Git" || first.ID != "Git.Git" || first.Current != "2.45.1" || first.Available != "2.46.0" || first.Source != "winget" {
		t.Fatalf("first = %+v", first)
	}
	if got[2].ID != "Microsoft.PowerShell" {
		t.Errorf("third = %+v", got[2])
	}
}

func TestParseUpgradeNothingToDo(t *testing.T) {
	for _, out := range []string{
		"No installed package found matching input criteria.\n",
		"No applicable upgrade found.\n",
	} {
		got, err := ParseUpgrade(out)
		if err != nil || len(got) != 0 {
			t.Errorf("ParseUpgrade(%q) = %v, %v", out, got, err)
		}
	}
}

func TestParseUpgradeUnrecognized(t *testing.T) {
	out := "Keine installierten Pakete gefunden.\n"
	if _, err := ParseUpgrade(out); !errors.Is(err, ErrUnparseable) {
		t.Fatalf("ParseUpgrade(localized) = %v, want ErrUnparseable", err)
	}
}

func TestParseUpgradeEmpty(t *testing.T) {
	if _, err := ParseUpgrade(""); !errors.Is(err, ErrUnparseable) {
		t.Fatalf("empty output: %v", err)
	}
}

func TestSliceColumns(t *testing.T) {
	header := "Name                                Id                                  Version      Available    Source"
	_, columns := findHeader([]string{header})
	row := "Git                                 Git.Git                             2.45.1       2.46.0       winget"
	fields := sliceColumns(row, columns)
	if got := trimAll(fields); got["Name"] != "Git" || got["Id"] != "Git.Git" || got["Available"] != "2.46.0" {
		t.Fatalf("fields = %v", got)
	}
}

func trimAll(m map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = trimSpace(v)
	}
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
