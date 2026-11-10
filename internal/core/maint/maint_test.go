package maint

import (
	"errors"
	"strings"
	"testing"
)

func TestTaskTableIsComplete(t *testing.T) {
	if len(Tasks) < 5 {
		t.Fatalf("only %d tasks", len(Tasks))
	}
	seen := map[string]bool{}
	for _, task := range Tasks {
		if task.Op == "" || task.Title == "" || task.Detail == "" {
			t.Errorf("incomplete task: %+v", task)
		}
		if seen[task.Op] {
			t.Errorf("duplicate op %q", task.Op)
		}
		seen[task.Op] = true
		if task.Timeout <= 0 {
			t.Errorf("%s has no timeout", task.Op)
		}
	}
}

func TestValidate(t *testing.T) {
	good := []Op{
		{Name: "dns.flush"},
		{Name: "cache.icon"},
		{Name: "startup.set-hklm", Args: map[string]string{"name": "Steam", "enabled": "false", "scope": "machine"}},
	}
	for _, op := range good {
		if err := Validate(op); err != nil {
			t.Errorf("Validate(%+v) = %v", op, err)
		}
	}

	bad := []Op{
		{Name: "registry.nuke"},
		{Name: "dns.flush", Args: map[string]string{"x": "y"}},
		{Name: "startup.set-hklm", Args: map[string]string{"name": "Steam"}},
		{Name: "startup.set-hklm", Args: map[string]string{"name": "Steam", "enabled": "maybe", "scope": "machine"}},
		{Name: "startup.set-hklm", Args: map[string]string{"name": "Steam", "enabled": "true", "scope": "user"}},
	}
	for _, op := range bad {
		if err := Validate(op); err == nil {
			t.Errorf("Validate(%+v) accepted", op)
		}
	}
	if err := Validate(Op{Name: "nope"}); !errors.Is(err, ErrUnknownOp) {
		t.Errorf("unknown op error = %v", err)
	}
}

func TestParseJobStrict(t *testing.T) {
	data, err := MarshalJob([]Op{{Name: "dns.flush"}})
	if err != nil {
		t.Fatal(err)
	}
	job, err := ParseJob(data)
	if err != nil {
		t.Fatal(err)
	}
	if job.V != 1 || len(job.Ops) != 1 || job.Ops[0].Name != "dns.flush" {
		t.Fatalf("job = %+v", job)
	}

	cases := map[string]string{
		"unknown field": `{"v":1,"ops":[{"op":"dns.flush"}],"extra":1}`,
		"wrong version": `{"v":2,"ops":[{"op":"dns.flush"}]}`,
		"empty ops":     `{"v":1,"ops":[]}`,
		"unknown op":    `{"v":1,"ops":[{"op":"format.c"}]}`,
		"bad args":      `{"v":1,"ops":[{"op":"startup.set-hklm","args":{"name":"x"}}]}`,
	}
	for name, body := range cases {
		if _, err := ParseJob([]byte(body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := ParseJob([]byte(strings.Repeat("x", MaxJobSize+1))); err == nil {
		t.Error("oversized job accepted")
	}
}

func TestMarshalJobRefusesUnknown(t *testing.T) {
	if _, err := MarshalJob([]Op{{Name: "format.c"}}); err == nil {
		t.Fatal("MarshalJob wrote an unknown op")
	}
}
