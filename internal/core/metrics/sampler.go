// Package metrics samples the machine's health once a second and keeps a
// short history for the sparklines. The sampler is written against a Reader
// so its arithmetic, rate calculation and history are testable without the
// operating system.
package metrics

import (
	"context"
	"sync"
	"time"
)

// Process is one row of the process table.
type Process struct {
	PID    int32
	Name   string
	CPU    float64 // percent of one core
	Memory uint64  // resident bytes
	User   string
}

// Raw is one reading from the OS. Counters are cumulative; the sampler turns
// them into rates.
type Raw struct {
	CPUPercent float64
	MemUsed    uint64
	MemTotal   uint64
	DiskRead   uint64
	DiskWrite  uint64
	NetRecv    uint64
	NetSent    uint64
	Uptime     time.Duration
	Processes  []Process
	// SysFree and SysTotal are the system drive's free and total bytes.
	SysFree, SysTotal uint64
	Battery           *Battery
	System            System
	// Cores is the share of each logical processor used, 0 to 100, in the
	// order Windows numbers them. Empty where the OS reports none.
	Cores []float64
}

// Reader reads the machine. GopsutilReader is the real one.
type Reader interface {
	Read(ctx context.Context) (Raw, error)
}

// History holds the last HistoryLen samples of each series, oldest first.
type History struct {
	CPU, Mem, Disk, Net []float64
	// NetDown and NetUp keep the two directions apart, for the network
	// card's two series; Net is their sum.
	NetDown, NetUp []float64
}

// HistoryLen is how many samples the sparklines show.
const HistoryLen = 60

// Snapshot is what the view draws. It is immutable once published.
type Snapshot struct {
	At         time.Time
	CPU        float64
	MemUsed    uint64
	MemTotal   uint64
	DiskRead   float64 // bytes per second
	DiskWrite  float64
	NetRecv    float64
	NetSent    float64
	Uptime     time.Duration
	Processes  []Process
	History    History
	Health     int // 0-100, see health
	SampleCost time.Duration
	SysFree    uint64
	SysTotal   uint64
	Battery    *Battery
	System     System
	Cores      []float64
}

// Sampler samples the reader every Interval until Stop. It is safe for
// concurrent use: Snapshot may be called from any goroutine.
type Sampler struct {
	reader   Reader
	interval time.Duration

	mu       sync.Mutex
	snapshot Snapshot
	rings    [6]*Ring

	prev    Raw
	hasPrev bool
	proc    map[int32]any // reserved for per-process rate state

	stop     chan struct{}
	stopOnce sync.Once
}

// NewSampler returns a sampler that reads every interval.
func NewSampler(reader Reader, interval time.Duration) *Sampler {
	if interval <= 0 {
		interval = time.Second
	}
	return &Sampler{
		reader:   reader,
		interval: interval,
		rings: [6]*Ring{
			NewRing(HistoryLen), // CPU
			NewRing(HistoryLen), // memory
			NewRing(HistoryLen), // disk
			NewRing(HistoryLen), // network
			NewRing(HistoryLen), // network down
			NewRing(HistoryLen), // network up
		},
		stop: make(chan struct{}),
		proc: map[int32]any{},
	}
}

// Start samples until ctx is done or Stop is called, calling onUpdate after
// every sample. onUpdate runs on the sampler's goroutine.
func (s *Sampler) Start(ctx context.Context, onUpdate func(Snapshot)) {
	s.tick(ctx, s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			s.tick(ctx, s.interval)
			if onUpdate != nil {
				onUpdate(s.Snapshot())
			}
		}
	}
}

// Stop ends Start.
func (s *Sampler) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
}

// Snapshot returns the latest sample.
func (s *Sampler) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.snapshot
	snap.History = History{
		CPU:     s.rings[0].values(nil),
		Mem:     s.rings[1].values(nil),
		Disk:    s.rings[2].values(nil),
		Net:     s.rings[3].values(nil),
		NetDown: s.rings[4].values(nil),
		NetUp:   s.rings[5].values(nil),
	}
	return snap
}

// tick reads once and turns counters into rates against the previous reading.
func (s *Sampler) tick(ctx context.Context, interval time.Duration) {
	start := time.Now()
	raw, err := s.reader.Read(ctx)
	cost := time.Since(start)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var diskRead, diskWrite, netRecv, netSent float64
	if s.hasPrev {
		seconds := interval.Seconds()
		diskRead = rate(raw.DiskRead, s.prev.DiskRead, seconds)
		diskWrite = rate(raw.DiskWrite, s.prev.DiskWrite, seconds)
		netRecv = rate(raw.NetRecv, s.prev.NetRecv, seconds)
		netSent = rate(raw.NetSent, s.prev.NetSent, seconds)
	}
	s.prev = raw
	s.hasPrev = true

	memPercent := 0.0
	if raw.MemTotal > 0 {
		memPercent = float64(raw.MemUsed) / float64(raw.MemTotal) * 100
	}
	s.rings[0].Push(raw.CPUPercent)
	s.rings[1].Push(memPercent)
	s.rings[2].Push(diskRead + diskWrite)
	s.rings[3].Push(netRecv + netSent)
	s.rings[4].Push(netRecv)
	s.rings[5].Push(netSent)

	s.snapshot = Snapshot{
		At:         time.Now(),
		CPU:        raw.CPUPercent,
		MemUsed:    raw.MemUsed,
		MemTotal:   raw.MemTotal,
		DiskRead:   diskRead,
		DiskWrite:  diskWrite,
		NetRecv:    netRecv,
		NetSent:    netSent,
		Uptime:     raw.Uptime,
		Processes:  raw.Processes,
		Health:     health(raw.CPUPercent, memPercent),
		SampleCost: cost,
		SysFree:    raw.SysFree,
		SysTotal:   raw.SysTotal,
		Battery:    raw.Battery,
		System:     raw.System,
		Cores:      raw.Cores,
	}
}

// rate converts two cumulative counters into per-second units, treating a
// counter that went backwards as no traffic rather than a huge spike.
func rate(now, before uint64, seconds float64) float64 {
	if now <= before || seconds <= 0 {
		return 0
	}
	return float64(now-before) / seconds
}

// health is 100 minus a penalty for sustained CPU and memory pressure. Each
// penalty is capped so one saturated resource cannot zero the score alone.
func health(cpuPercent, memPercent float64) int {
	penalty := 0.0
	if cpuPercent > 85 {
		penalty += min(40, cpuPercent-85)
	}
	if memPercent > 90 {
		penalty += 40
	} else if memPercent > 75 {
		penalty += 15
	}
	score := 100 - penalty
	if score < 0 {
		score = 0
	}
	return int(score)
}
