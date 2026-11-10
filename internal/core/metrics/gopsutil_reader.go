package metrics

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// GopsutilReader reads the machine through gopsutil, which needs no cgo on
// Windows. It keeps process handles between reads so per-process CPU is a
// real percentage rather than the zero a fresh handle reports.
type GopsutilReader struct {
	procs     map[int32]*process.Process
	procsAt   time.Time
	procEvery time.Duration
	topN      int

	systemOnce sync.Once
	system     System
}

// NewGopsutilReader returns a reader that refreshes the process table every
// two seconds and keeps its top 40 by CPU.
func NewGopsutilReader() *GopsutilReader {
	return &GopsutilReader{procs: map[int32]*process.Process{}, procEvery: 2 * time.Second, topN: 40}
}

// Read implements Reader.
func (g *GopsutilReader) Read(ctx context.Context) (Raw, error) {
	var raw Raw
	// One per-core read: the percentages are since the last call, so a
	// second aggregate call would see no time at all. The overall figure is
	// their mean, which is how Windows reports it.
	if percents, err := cpu.PercentWithContext(ctx, 0, true); err == nil && len(percents) > 0 {
		raw.Cores = percents
		sum := 0.0
		for _, p := range percents {
			sum += p
		}
		raw.CPUPercent = sum / float64(len(percents))
	}
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		raw.MemUsed = vm.Used
		raw.MemTotal = vm.Total
	}
	if counters, err := disk.IOCountersWithContext(ctx); err == nil {
		for _, c := range counters {
			raw.DiskRead += c.ReadBytes
			raw.DiskWrite += c.WriteBytes
		}
	}
	if counters, err := net.IOCountersWithContext(ctx, false); err == nil && len(counters) > 0 {
		raw.NetRecv = counters[0].BytesRecv
		raw.NetSent = counters[0].BytesSent
	}
	if uptime, err := host.UptimeWithContext(ctx); err == nil {
		raw.Uptime = time.Duration(uptime) * time.Second
	}
	if usage, err := disk.UsageWithContext(ctx, systemDrive()); err == nil {
		raw.SysFree = usage.Free
		raw.SysTotal = usage.Total
	}
	raw.Battery = readBattery()
	g.systemOnce.Do(func() { g.system = readSystem(ctx) })
	raw.System = g.system
	raw.Processes = g.processes(ctx)
	return raw, nil
}

// systemDrive is the drive Windows runs from, as C:\.
func systemDrive() string {
	if d := os.Getenv("SystemDrive"); d != "" {
		return d + `\`
	}
	return `C:\`
}

// readSystem reads the facts that do not change: the processor's name, which
// costs a WMI query, is read once.
func readSystem(ctx context.Context) System {
	sys := System{OS: osLabel()}
	if n, err := cpu.CountsWithContext(ctx, true); err == nil {
		sys.Cores = n
	}
	if infos, err := cpu.InfoWithContext(ctx); err == nil && len(infos) > 0 {
		sys.CPUModel = shortCPU(infos[0].ModelName)
	}
	return sys
}

func (g *GopsutilReader) processes(ctx context.Context) []Process {
	if time.Since(g.procsAt) >= g.procEvery {
		g.refresh(ctx)
		g.procsAt = time.Now()
	}
	out := make([]Process, 0, len(g.procs))
	for pid, p := range g.procs {
		row := Process{PID: pid}
		if name, err := p.NameWithContext(ctx); err == nil {
			row.Name = name
		}
		if percent, err := p.CPUPercentWithContext(ctx); err == nil {
			row.CPU = percent
		}
		if info, err := p.MemoryInfoWithContext(ctx); err == nil && info != nil {
			row.Memory = info.RSS
		}
		if user, err := p.UsernameWithContext(ctx); err == nil {
			row.User = user
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CPU != out[j].CPU {
			return out[i].CPU > out[j].CPU
		}
		return out[i].Memory > out[j].Memory
	})
	if len(out) > g.topN {
		out = out[:g.topN]
	}
	return out
}

// refresh drops processes that ended and adds new ones. It is called from the
// sampler's goroutine only.
func (g *GopsutilReader) refresh(ctx context.Context) {
	live, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return
	}
	seen := make(map[int32]*process.Process, len(live))
	for _, p := range live {
		if cached, ok := g.procs[p.Pid]; ok {
			seen[p.Pid] = cached
			continue
		}
		seen[p.Pid] = p
	}
	g.procs = seen
}

// DriveUsage returns the used and total bytes of the volume holding path.
func DriveUsage(path string) (used, total uint64, err error) {
	root := filepath.VolumeName(path)
	if root == "" {
		root = path
	} else {
		root += `\`
	}
	usage, err := disk.Usage(root)
	if err != nil {
		return 0, 0, err
	}
	return usage.Used, usage.Total, nil
}
