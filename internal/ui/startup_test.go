package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/maint"
	"elom/internal/core/startup"
	"elom/locales"
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

func TestStartupToggleFailureIsNotAReadError(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.appsTab = 2
	a.startup.loaded = true
	a.startup.entries = []startup.Entry{
		{Name: "Discord", Source: "HKCU Run", Scope: "user", Enabled: false, Writable: true},
	}
	a.startup.toggleErr = locales.StartupToggleFailed("Discord", "access denied")

	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "Could not change Discord: access denied") {
		t.Errorf("toggle failure not named: %q", texts)
	}
	if strings.Contains(texts, "Could not read startup entries") {
		t.Errorf("toggle failure rendered as the list's read error: %q", texts)
	}
}

func TestStartupMachineEntryExplainsAdmin(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "software"
	a.appsTab = 2
	a.startup.loaded = true
	a.startup.entries = []startup.Entry{
		{Name: "Steam", Source: "HKLM Run", Scope: "machine", Enabled: true, Writable: true},
	}
	tt := NewTester(a.View, 1180, 760)
	if !tt.HasText("needs administrator") {
		t.Errorf("machine-wide entry does not explain its prompt: %q", tt.Texts())
	}
	if !tt.HasText("HKLM Run") {
		t.Errorf("localized source missing: %q", tt.Texts())
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
