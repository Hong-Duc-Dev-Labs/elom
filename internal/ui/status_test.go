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
