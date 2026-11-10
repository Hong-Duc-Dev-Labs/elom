package main

import "testing"

func TestElevatedJobArg(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"Elom.exe"}, ""},
		{[]string{"Elom.exe", "--elevated-job"}, ""},
		{[]string{"Elom.exe", "--elevated-job", "job.json"}, "job.json"},
		{[]string{"--elevated-job", "", "x"}, ""},
	}
	for _, c := range cases {
		if got := elevatedJobArg(c.args); got != c.want {
			t.Errorf("elevatedJobArg(%q) = %q, want %q", c.args, got, c.want)
		}
	}
}
