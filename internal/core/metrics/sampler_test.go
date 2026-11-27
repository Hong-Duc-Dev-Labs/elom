package metrics

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestRingWraps(t *testing.T) {
	r := NewRing(3)
	if r.Len() != 0 {
		t.Fatalf("empty ring Len = %d", r.Len())
	}
	r.Push(1)
	r.Push(2)
	if got := r.Values(); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("values = %v", got)
	}
	r.Push(3)
	r.Push(4) // overwrites 1
	got := r.Values()
	want := []float64{2, 3, 4}
	if len(got) != 3 {
		t.Fatalf("values = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("values = %v, want %v", got, want)
		}
	}
	if r.Len() != 3 {
		t.Fatalf("Len = %d", r.Len())
	}
}

func TestRingReusesBuffer(t *testing.T) {
	r := NewRing(2)
	r.Push(1)
	r.Push(2)
	buf := make([]float64, 0, 4)
	got := r.values(buf)
	if cap(got) != 4 {
		t.Fatalf("values allocated a new buffer: cap %d", cap(got))
	}
}

type fakeReader struct {
	raws []Raw
	i    int
}

func (f *fakeReader) Read(context.Context) (Raw, error) {
	if f.i >= len(f.raws) {
		return Raw{}, errors.New("done")
	}
	raw := f.raws[f.i]
	f.i++
	return raw, nil
}

func TestSamplerRatesAndHistory(t *testing.T) {
	reader := &fakeReader{raws: []Raw{
		{CPUPercent: 10, MemUsed: 100, MemTotal: 1000, SwapUsed: 25, SwapTotal: 500, DiskRead: 1000, NetRecv: 500, Processes: []Process{{PID: 1, Name: "a"}}},
		{CPUPercent: 20, MemUsed: 200, MemTotal: 1000, SwapUsed: 50, SwapTotal: 500, DiskRead: 3000, NetRecv: 2500, Processes: []Process{{PID: 2, Name: "b"}}},
	}}
	s := NewSampler(reader, time.Second)
	s.tick(context.Background(), time.Second)
	first := s.Snapshot()
	if first.CPU != 10 || first.DiskRead != 0 {
		t.Fatalf("first snapshot = %+v", first)
	}
	if len(first.History.CPU) != 1 || first.History.CPU[0] != 10 {
		t.Fatalf("history = %+v", first.History)
	}

	s.tick(context.Background(), time.Second)
	second := s.Snapshot()
	if second.CPU != 20 {
		t.Errorf("CPU = %v, want 20", second.CPU)
	}
	if second.DiskRead != 2000 || second.NetRecv != 2000 {
		t.Errorf("rates = disk %.0f net %.0f, want 2000 each", second.DiskRead, second.NetRecv)
	}
	if len(second.History.CPU) != 2 || second.History.CPU[1] != 20 {
		t.Errorf("history = %v", second.History.CPU)
	}
	if got := second.History.Mem[1]; got < 19.9 || got > 20.1 {
		t.Errorf("memory history = %v, want ~20%%", second.History.Mem)
	}
	if second.Processes[0].PID != 2 {
		t.Errorf("processes = %+v", second.Processes)
	}
	if second.Health < 0 || second.Health > 100 {
		t.Errorf("health = %d", second.Health)
	}
	if second.SwapUsed != 50 || second.SwapTotal != 500 {
		t.Errorf("swap = %d/%d, want 50/500", second.SwapUsed, second.SwapTotal)
	}
}

func TestSamplerIgnoresBackwardsCounters(t *testing.T) {
	reader := &fakeReader{raws: []Raw{
		{DiskRead: 5000},
		{DiskRead: 1000}, // reset or wrap
	}}
	s := NewSampler(reader, time.Second)
	s.tick(context.Background(), time.Second)
	s.tick(context.Background(), time.Second)
	if got := s.Snapshot().DiskRead; got != 0 {
		t.Fatalf("DiskRead = %v, want 0 for a counter that went backwards", got)
	}
}

func TestSamplerSurvivesReadError(t *testing.T) {
	reader := &fakeReader{}
	s := NewSampler(reader, time.Second)
	s.tick(context.Background(), time.Second) // fake returns an error
	if got := s.Snapshot(); got.CPU != 0 || len(got.History.CPU) != 0 {
		t.Fatalf("snapshot after an error = %+v", got)
	}
	s.tick(context.Background(), time.Second)
}

func TestHealth(t *testing.T) {
	if got := healthScore(10, 40, 100_000_000_000, 500_000_000_000, 0, time.Hour); got != 100 {
		t.Errorf("idle healthScore = %d, want 100", got)
	}
	// High CPU lowers score monotonically.
	low := healthScore(80, 40, 100_000_000_000, 500_000_000_000, 0, time.Hour)
	high := healthScore(95, 40, 100_000_000_000, 500_000_000_000, 0, time.Hour)
	if !(high < low && low < 100) {
		t.Errorf("CPU health not monotonic: 80%%=%d, 95%%=%d", low, high)
	}
	// Disk at 92% used with 200 GB free (2.5 TB drive) is NOT critical, whereas
	// 92% used with 10 GB free IS critical.
	bigDrive := healthScore(10, 40, 200_000_000_000, 2_500_000_000_000, 0, time.Hour)
	smallDrive := healthScore(10, 40, 10_000_000_000, 125_000_000_000, 0, time.Hour)
	if diskIsCritical(200_000_000_000, 2_500_000_000_000) {
		t.Error("2.5 TB drive with 200 GB free flagged critical")
	}
	if !diskIsCritical(10_000_000_000, 125_000_000_000) {
		t.Error("125 GB drive with 10 GB free not flagged critical")
	}
	if !(smallDrive < bigDrive) {
		t.Errorf("critical small drive (%d) should score lower than big drive (%d)", smallDrive, bigDrive)
	}
	// Heavy disk I/O and long uptime penalize health.
	if got := healthScore(10, 40, 100_000_000_000, 500_000_000_000, 200_000_000, 15*24*time.Hour); got != 87 {
		t.Errorf("heavy IO + 15d uptime healthScore = %d, want 87", got)
	}
}

func TestDiagnose(t *testing.T) {
	if got := diagnose(Snapshot{CPU: 15, MemUsed: 4, MemTotal: 16}); got.Kind != "ok" {
		t.Errorf("idle diagnose = %+v, want ok", got)
	}
	got := diagnose(Snapshot{
		CPU: 92,
		Processes: []Process{
			{PID: 1, Name: "chrome.exe", CPU: 75},
			{PID: 2, Name: "idle.exe", CPU: 2},
		},
	})
	if got.Kind != "cpu" || got.Target != "chrome.exe" {
		t.Errorf("cpu diagnose = %+v, want cpu/chrome.exe", got)
	}
	got = diagnose(Snapshot{
		CPU:      20,
		MemUsed:  95,
		MemTotal: 100,
		Processes: []Process{
			{PID: 1, Name: "Code.exe", Memory: 8_000_000_000},
		},
	})
	if got.Kind != "memory" || got.Target != "Code.exe" {
		t.Errorf("memory diagnose = %+v, want memory/Code.exe", got)
	}
	got = diagnose(Snapshot{
		CPU:      10,
		SysFree:  10_000_000_000,
		SysTotal: 200_000_000_000,
	})
	if got.Kind != "disk" || got.FreeBytes != 10_000_000_000 {
		t.Errorf("disk diagnose = %+v, want disk", got)
	}
}

func TestComputeProcessesGuardsPIDReuseAndLimitsUserLookups(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 8, 0, 0, 0, time.UTC)
	t1 := t0.Add(2 * time.Second)
	prev := map[int32]procSample{
		10: {created: 1000, total: 5.0, at: t0},
		20: {created: 2000, total: 10.0, at: t0}, // will be reused PID with different created
	}
	samples := []rawProcSample{
		{pid: 10, name: "busy.exe", created: 1000, total: 7.0, hasCPU: true, rss: 100, at: t1},  // delta 2s / 2s = 100%
		{pid: 20, name: "new.exe", created: 9999, total: 50.0, hasCPU: true, rss: 200, at: t1},  // PID reused -> 0%
		{pid: 30, name: "idle.exe", created: 3000, total: 1.0, hasCPU: true, rss: 50, at: t1},   // first sample -> 0%
	}
	lookups := 0
	procs, next := computeProcesses(samples, prev, 2, func(pid int32) string {
		lookups++
		return fmt.Sprintf("user-%d", pid)
	})
	if len(procs) != 2 {
		t.Fatalf("len(procs) = %d, want 2", len(procs))
	}
	if lookups != 2 {
		t.Errorf("lookupUser called %d times, want 2 (only topN)", lookups)
	}
	if procs[0].PID != 10 || procs[0].CPU < 99.9 || procs[0].CPU > 100.1 || procs[0].User != "user-10" {
		t.Errorf("top proc = %+v, want PID 10 at ~100%% CPU", procs[0])
	}
	if procs[1].PID != 20 || procs[1].CPU != 0 {
		t.Errorf("reused PID proc = %+v, want 0%% CPU", procs[1])
	}
	if len(next) != 3 {
		t.Errorf("next samples = %d, want 3", len(next))
	}
}

// TestRealSample is opt-in: it reads this machine through gopsutil, proving
// the Windows coverage and measuring the cost. Set ELOM_REAL_SAMPLE=1.
func TestRealSample(t *testing.T) {
	if os.Getenv("ELOM_REAL_SAMPLE") == "" {
		t.Skip("set ELOM_REAL_SAMPLE=1 to read the real machine")
	}
	reader := NewGopsutilReader()
	start := time.Now()
	first, err := reader.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	firstCost := time.Since(start)
	if first.MemTotal == 0 {
		t.Fatal("no memory total")
	}
	time.Sleep(time.Second)
	start = time.Now()
	second, err := reader.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cpu %.1f%% mem %s/%s disk %d/%d net %d/%d procs %d first-read %s second-read %s",
		second.CPUPercent, humanBytes(int64(second.MemUsed)), humanBytes(int64(second.MemTotal)),
		second.DiskRead, second.DiskWrite, second.NetRecv, second.NetSent,
		len(second.Processes), firstCost.Round(time.Millisecond), time.Since(start).Round(time.Millisecond))
	if len(second.Processes) == 0 {
		t.Fatal("no processes")
	}
	t.Logf("top: %s %.1f%% %s", second.Processes[0].Name, second.Processes[0].CPU, humanBytes(int64(second.Processes[0].Memory)))
}

func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}
