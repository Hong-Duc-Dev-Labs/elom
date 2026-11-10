package startup

import "testing"

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
