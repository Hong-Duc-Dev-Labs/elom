package ui

import (
	"strings"
	"testing"
	"time"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/metrics"
)

func sampleSnapshot() metrics.Snapshot {
	return metrics.Snapshot{
		At:        time.Now(),
		CPU:       12,
		MemUsed:   4_000_000_000,
		MemTotal:  16_000_000_000,
		DiskRead:  1_000_000,
		DiskWrite: 500_000,
		NetRecv:   2_000_000,
		NetSent:   300_000,
		Uptime:    50*time.Hour + 5*time.Minute,
		Health:    100,
		SysFree:   120_000_000_000,
		SysTotal:  512_000_000_000,
		Cores:     []float64{12, 30, 8, 44, 20, 6, 55, 18, 9, 33, 14, 22},
		System:    metrics.System{Cores: 12},
		Processes: []metrics.Process{
			{PID: 42, Name: "notepad.exe", CPU: 3.5, Memory: 50_000_000},
			{PID: 43, Name: "explorer.exe", CPU: 1.0, Memory: 200_000_000},
		},
		History: metrics.History{
			CPU:  []float64{10, 12, 14},
			Mem:  []float64{20, 22, 25},
			Disk:    []float64{1000, 2000, 1500},
			Net:     []float64{500, 800, 1200},
			NetDown: []float64{400, 600, 900},
			NetUp:   []float64{100, 200, 300},
		},
	}
}

func TestStatusPageRenders(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "status"
	a.status = sampleSnapshot()

	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"CPU", "12", "MEMORY", "notepad.exe", "HEALTH", "100", "2d 2h 5m", "BUSIEST PROCESSES"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
}

func TestStatusProcessTableExplainsEmptyList(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "status"
	snap := sampleSnapshot()
	snap.Processes = nil
	a.status = snap
	tt := NewTester(a.View, 1180, 760)
	if !tt.HasText("No processes are listed right now.") {
		t.Errorf("empty process table has no reason: %q", tt.Texts())
	}
}

func TestStatusShowsKillFailure(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "status"
	a.status = sampleSnapshot()
	a.statusKillErr = "Could not end notepad.exe: access is denied."
	tt := NewTester(a.View, 1180, 760)
	if !tt.HasText("Could not end notepad.exe: access is denied.") {
		t.Errorf("kill failure not shown: %q", tt.Texts())
	}
}

func TestStatusPageWaitsForFirstSample(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "status"
	tt := NewTester(a.View, 1180, 760)
	if !tt.HasText("Starting the sampler…") {
		t.Errorf("no waiting state: %q", tt.Texts())
	}
}

func TestStatusPageRendersDiagnosisAndMultiDrive(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "status"
	snap := sampleSnapshot()
	snap.Diagnosis = metrics.Diagnosis{Kind: "cpu", Target: "chrome.exe"}
	snap.Drives = []metrics.Drive{
		{Mount: "C:", Total: 512_000_000_000, Free: 120_000_000_000},
		{Mount: "D:", Total: 1_000_000_000_000, Free: 450_000_000_000},
	}
	a.status = snap

	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"chrome.exe high CPU", "C: 120.0 GB free", "D: 450.0 GB free"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	if got := formatUptime(90 * time.Minute); got != "1h 30m" {
		t.Errorf("formatUptime = %q", got)
	}
	if got := formatUptime(50*time.Hour + 5*time.Minute); got != "2d 2h 5m" {
		t.Errorf("formatUptime = %q", got)
	}
}

func TestSeriesMax(t *testing.T) {
	if got := seriesMax(nil); got != 1024 {
		t.Errorf("empty series max = %v", got)
	}
	if got := seriesMax([]float64{5, 9000, 3}); got != 9000 {
		t.Errorf("series max = %v", got)
	}
}

func TestStatusSwapDisplayAndControls(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "status"
	snap := sampleSnapshot()
	snap.SwapUsed = 1_500_000_000
	snap.SwapTotal = 4_000_000_000
	a.status = snap

	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"Swap 1.5 GB / 4.0 GB", "Pause", "CPU", "Memory"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
}

func TestStatusPauseFreezesDisplayedSnapshot(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "status"
	snap1 := sampleSnapshot()
	snap1.CPU = 18
	a.status = snap1

	a.toggleStatusPause()
	if !a.statusPaused {
		t.Fatal("expected statusPaused to be true")
	}

	// Background sampler updates a.status while paused.
	snap2 := sampleSnapshot()
	snap2.CPU = 91
	a.status = snap2

	if got := a.displayedStatus().CPU; got != 18 {
		t.Errorf("displayedStatus().CPU = %.0f while paused, want 18", got)
	}
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"Paused", "Resume"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}

	// Resuming immediately exposes the latest live sample.
	a.toggleStatusPause()
	if got := a.displayedStatus().CPU; got != 91 {
		t.Errorf("displayedStatus().CPU = %.0f after resume, want 91", got)
	}
}

func TestSortedProcessesCPUAndMemory(t *testing.T) {
	procs := []metrics.Process{
		{PID: 1, Name: "cpu-heavy.exe", CPU: 45, Memory: 100},
		{PID: 2, Name: "mem-heavy.exe", CPU: 5, Memory: 900},
	}
	byCPU := sortedProcesses(procs, false)
	if byCPU[0].Name != "cpu-heavy.exe" {
		t.Errorf("CPU sort put %q first", byCPU[0].Name)
	}
	byMem := sortedProcesses(procs, true)
	if byMem[0].Name != "mem-heavy.exe" {
		t.Errorf("Memory sort put %q first", byMem[0].Name)
	}
}

func TestUIThrottleLimitsRate(t *testing.T) {
	th := newUIThrottle(40 * time.Millisecond)
	if !th.allow() {
		t.Fatal("first call must be allowed")
	}
	if th.allow() {
		t.Fatal("immediate second call must be throttled")
	}
	time.Sleep(50 * time.Millisecond)
	if !th.allow() {
		t.Fatal("call after interval must be allowed")
	}
}
