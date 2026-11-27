package startup

import (
	"strings"
	"testing"
)

func TestApprovedEnabled(t *testing.T) {
	cases := []struct {
		blob []byte
		want bool
	}{
		{nil, true},
		{[]byte{}, true},
		{[]byte{2, 0, 0}, true},
		{[]byte{3, 0, 0}, false},
		{ApprovedBlob(true), true},
		{ApprovedBlob(false), false},
	}
	for _, c := range cases {
		if got := ApprovedEnabled(c.blob); got != c.want {
			t.Errorf("ApprovedEnabled(%v) = %v, want %v", c.blob, got, c.want)
		}
	}
}

func TestApprovedBlob(t *testing.T) {
	on := ApprovedBlob(true)
	if len(on) != 12 || on[0] != 2 {
		t.Fatalf("enabled blob = %v", on)
	}
	off := ApprovedBlob(false)
	if len(off) != 12 || off[0] != 3 {
		t.Fatalf("disabled blob = %v", off)
	}
	for _, b := range on[1:] {
		if b != 0 {
			t.Fatalf("enabled blob has payload: %v", on)
		}
	}
}

func TestParseScheduledTasksCSV(t *testing.T) {
	raw := strings.Join([]string{
		`"TaskName","Status","Schedule Type","Task To Run"`,
		`"\Vendor\HelperSync","Ready","At logon time","C:\Tools\helper.exe --tray"`,
		`"\Vendor\PausedAgent","Disabled","At log on","C:\Tools\agent.exe"`,
		`"\Vendor\NightlyBackup","Ready","Daily","C:\Tools\backup.exe"`,
		`"\Microsoft\Windows\Shell\Indexer","Ready","At logon time","C:\Windows\System32\idx.exe"`,
	}, "\r\n")

	got := ParseScheduledTasksCSV([]byte(raw))
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(got), got)
	}
	if got[0].Name != "HelperSync" || !got[0].Enabled || got[0].Writable || got[0].Source != SourceScheduledTask {
		t.Errorf("first entry = %+v", got[0])
	}
	if got[1].Name != "PausedAgent" || got[1].Enabled || got[1].Writable {
		t.Errorf("second entry = %+v", got[1])
	}
}
