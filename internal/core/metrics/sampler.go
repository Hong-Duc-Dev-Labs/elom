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

// Drive describes one fixed local volume (e.g. C:, D:).
type Drive struct {
	Mount       string
	Total       uint64
	Used        uint64
	Free        uint64
	UsedPercent float64
	FSType      string
}

// Diagnosis summarizes the primary system bottleneck for the Status health
// tile. Kind is one of "ok", "cpu", "memory", "disk", "disk_io", "uptime".
type Diagnosis struct {
	Kind      string
	Target    string
	FreeBytes uint64
}

// Raw is one reading from the OS. Counters are cumulative; the sampler turns
// them into rates.
type Raw struct {
	CPUPercent float64
	MemUsed    uint64
	MemTotal   uint64
	SwapUsed   uint64
	SwapTotal  uint64
	DiskRead   uint64
	DiskWrite  uint64
	NetRecv    uint64
	NetSent    uint64
	Uptime     time.Duration
	Processes  []Process
	// SysFree and SysTotal are the system drive's free and total bytes.
	SysFree, SysTotal uint64
	// Drives lists all fixed local volumes with drive letters (C:, D:, …).
	Drives  []Drive
	Battery *Battery
	System  System
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
	SwapUsed   uint64
	SwapTotal  uint64
	DiskRead   float64 // bytes per second
	DiskWrite  float64
	NetRecv    float64
	NetSent    float64
	Uptime     time.Duration
	Processes  []Process
	History    History
	Health     int // 0-100, see healthScore
	Diagnosis  Diagnosis
	SampleCost time.Duration
	SysFree    uint64
	SysTotal   uint64
	Drives     []Drive
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

	snap := Snapshot{
		At:         time.Now(),
		CPU:        raw.CPUPercent,
		MemUsed:    raw.MemUsed,
		MemTotal:   raw.MemTotal,
		SwapUsed:   raw.SwapUsed,
		SwapTotal:  raw.SwapTotal,
		DiskRead:   diskRead,
		DiskWrite:  diskWrite,
		NetRecv:    netRecv,
		NetSent:    netSent,
		Uptime:     raw.Uptime,
		Processes:  raw.Processes,
		Health:     healthScore(raw.CPUPercent, memPercent, raw.SysFree, raw.SysTotal, diskRead+diskWrite, raw.Uptime),
		SampleCost: cost,
		SysFree:    raw.SysFree,
		SysTotal:   raw.SysTotal,
		Drives:     raw.Drives,
		Battery:    raw.Battery,
		System:     raw.System,
		Cores:      raw.Cores,
	}
	snap.Diagnosis = diagnose(snap)
	s.snapshot = snap
}

// rate converts two cumulative counters into per-second units, treating a
// counter that went backwards as no traffic rather than a huge spike.
func rate(now, before uint64, seconds float64) float64 {
	if now <= before || seconds <= 0 {
		return 0
	}
	return float64(now-before) / seconds
}

const (
	diskCriticalPercent   = 90.0
	diskCriticalFreeBytes = 30_000_000_000 // 30 GB
	ioWarnBytesPerSec     = 50_000_000     // 50 MB/s
	ioHighBytesPerSec     = 150_000_000    // 150 MB/s
	uptimeWarn            = 7 * 24 * time.Hour
	uptimeDanger          = 14 * 24 * time.Hour
)

// diskIsCritical reports a volume at least 90% full with under 30 GB free:
// a 2 TB disk at 90% still has 200 GB free and should not page the user.
func diskIsCritical(free, total uint64) bool {
	if total == 0 || free >= total {
		return false
	}
	usedPct := float64(total-free) / float64(total) * 100
	return usedPct >= diskCriticalPercent && free < diskCriticalFreeBytes
}

// healthScore combines CPU, memory, system-disk headroom, disk I/O and uptime
// into a 0-100 score. Each factor is capped so one busy resource cannot zero
// the score alone.
func healthScore(cpuPercent, memPercent float64, sysFree, sysTotal uint64, diskIO float64, uptime time.Duration) int {
	score := 100.0

	if cpuPercent > 50 {
		if cpuPercent > 85 {
			score -= 30.0 * (cpuPercent - 50) / 50
		} else {
			score -= 15.0 * (cpuPercent - 50) / 35
		}
	}

	if memPercent > 70 {
		if memPercent > 88 {
			score -= 25.0 * (memPercent - 70) / 30
		} else {
			score -= 12.5 * (memPercent - 70) / 18
		}
	}

	if sysTotal > 0 && sysFree < sysTotal {
		usedPct := float64(sysTotal-sysFree) / float64(sysTotal) * 100
		if usedPct > 80 {
			if diskIsCritical(sysFree, sysTotal) {
				score -= 20.0 * (usedPct - 80) / 20
			} else {
				score -= min(10.0, 10.0*(usedPct-80)/13)
			}
		}
	}

	if diskIO > ioWarnBytesPerSec {
		if diskIO >= ioHighBytesPerSec {
			score -= 10.0
		} else {
			score -= 10.0 * (diskIO - ioWarnBytesPerSec) / (ioHighBytesPerSec - ioWarnBytesPerSec)
		}
	}

	if uptime > uptimeDanger {
		score -= 3
	} else if uptime > uptimeWarn {
		score -= 1
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return int(score)
}

// diagnose names the primary bottleneck in snap, or "ok" when all clear.
func diagnose(snap Snapshot) Diagnosis {
	if snap.CPU > 85 {
		if proc, ok := leadingCPUProcess(snap.Processes, 50); ok {
			return Diagnosis{Kind: "cpu", Target: proc.Name}
		}
		return Diagnosis{Kind: "cpu"}
	}
	memPct := 0.0
	if snap.MemTotal > 0 {
		memPct = float64(snap.MemUsed) / float64(snap.MemTotal) * 100
	}
	if memPct > 88 {
		if proc, ok := leadingMemoryProcess(snap.Processes); ok && proc.Memory > 0 {
			return Diagnosis{Kind: "memory", Target: proc.Name}
		}
		return Diagnosis{Kind: "memory"}
	}
	if diskIsCritical(snap.SysFree, snap.SysTotal) {
		return Diagnosis{Kind: "disk", FreeBytes: snap.SysFree}
	}
	if snap.DiskRead+snap.DiskWrite >= ioHighBytesPerSec {
		return Diagnosis{Kind: "disk_io"}
	}
	if snap.Uptime > uptimeDanger {
		return Diagnosis{Kind: "uptime"}
	}
	return Diagnosis{Kind: "ok"}
}

func leadingCPUProcess(procs []Process, threshold float64) (Process, bool) {
	var best Process
	for i, p := range procs {
		if i == 0 || p.CPU > best.CPU {
			best = p
		}
	}
	if len(procs) == 0 || best.CPU < threshold {
		return Process{}, false
	}
	return best, true
}

func leadingMemoryProcess(procs []Process) (Process, bool) {
	var best Process
	for i, p := range procs {
		if i == 0 || p.Memory > best.Memory {
			best = p
		}
	}
	return best, len(procs) > 0
}
