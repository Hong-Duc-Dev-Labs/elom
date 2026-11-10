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
		{CPUPercent: 10, MemUsed: 100, MemTotal: 1000, DiskRead: 1000, NetRecv: 500, Processes: []Process{{PID: 1, Name: "a"}}},
		{CPUPercent: 20, MemUsed: 200, MemTotal: 1000, DiskRead: 3000, NetRecv: 2500, Processes: []Process{{PID: 2, Name: "b"}}},
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
	cases := []struct {
		cpu, mem float64
		want     int
	}{
		{10, 40, 100},
		{95, 40, 90},
		{99, 40, 86},
		{10, 80, 85},
		{10, 95, 60},
		{100, 100, 45},
	}
	for _, c := range cases {
		if got := health(c.cpu, c.mem); got != c.want {
			t.Errorf("health(%v, %v) = %d, want %d", c.cpu, c.mem, got, c.want)
		}
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
