package ui

import (
	"context"
	"testing"
	"time"

	"elom/internal/core/maint"
	"elom/internal/core/startup"
)

func TestStartupOpPayload(t *testing.T) {
	op := startupOp(startup.Entry{Name: "Steam"}, false)
	if op.Name != "startup.set-hklm" {
		t.Fatalf("op = %q", op.Name)
	}
	if op.Args["name"] != "Steam" || op.Args["enabled"] != "false" || op.Args["scope"] != "machine" {
		t.Fatalf("args = %v", op.Args)
	}
	if err := maint.Validate(op); err != nil {
		t.Fatalf("the UI would send an op the job rejects: %v", err)
	}
}

func TestMachineEntryGoesThroughElevation(t *testing.T) {
	a := newTestApp(t)
	a.startup.loaded = true
	a.startup.entries = []startup.Entry{
		{Name: "Steam", Source: "HKLM Run", Scope: "machine", Enabled: true, Writable: true},
	}
	sent := make(chan maint.Op, 1)
	a.Elevate = func(ctx context.Context, ops []maint.Op) ([]maint.Result, error) {
		sent <- ops[0]
		return []maint.Result{{Op: ops[0].Name, OK: true}}, nil
	}
	a.toggleStartup(0, false)
	select {
	case op := <-sent:
		if op.Args["name"] != "Steam" || op.Args["enabled"] != "false" {
			t.Fatalf("op = %+v", op)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the machine entry was not routed through elevation")
	}
}
