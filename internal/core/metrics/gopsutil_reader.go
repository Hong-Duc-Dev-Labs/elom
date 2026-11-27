package metrics

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type procSample struct {
	created int64
	total   float64
	at      time.Time
}

type rawProcSample struct {
	pid     int32
	name    string
	created int64
	total   float64
	hasCPU  bool
	rss     uint64
	at      time.Time
	handle  *process.Process
}

// GopsutilReader reads the machine through gopsutil, which needs no cgo on
// Windows. It refreshes the process list on procEvery (2s), computes per-process
// CPU deltas guarded by CreateTime to prevent PID-reuse artifacts, and resolves
// usernames only for the topN processes displayed.
type GopsutilReader struct {
	prevSamples map[int32]procSample
	cachedProcs []Process
	procsAt     time.Time
	procEvery   time.Duration
	topN        int

	systemOnce sync.Once
	system     System
}

// NewGopsutilReader returns a reader that refreshes the process table every
// two seconds and keeps its top 40 by CPU.
func NewGopsutilReader() *GopsutilReader {
	return &GopsutilReader{
		prevSamples: map[int32]procSample{},
		procEvery:   2 * time.Second,
		topN:        40,
	}
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
	if sw, err := mem.SwapMemoryWithContext(ctx); err == nil {
		raw.SwapUsed = sw.Used
		raw.SwapTotal = sw.Total
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
	raw.Drives = readDrives(ctx)
	sysDrive := strings.ToUpper(strings.TrimRight(systemDrive(), `\`))
	for _, d := range raw.Drives {
		if strings.EqualFold(strings.TrimRight(d.Mount, `\`), sysDrive) {
			raw.SysFree = d.Free
			raw.SysTotal = d.Total
			break
		}
	}
	if raw.SysTotal == 0 {
		if usage, err := disk.UsageWithContext(ctx, systemDrive()); err == nil {
			raw.SysFree = usage.Free
			raw.SysTotal = usage.Total
		}
	}
	raw.Battery = readBattery()
	g.systemOnce.Do(func() { g.system = readSystem(ctx) })
	raw.System = g.system
	raw.Processes = g.processes(ctx)
	return raw, nil
}

// readDrives enumerates fixed local drive letters (A:..Z:).
func readDrives(ctx context.Context) []Drive {
	parts, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var drives []Drive
	for _, p := range parts {
		if len(p.Device) < 2 || p.Device[1] != ':' {
			continue
		}
		letter := strings.ToUpper(p.Device[:2])
		if seen[letter] {
			continue
		}
		seen[letter] = true
		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil || usage == nil || usage.Total == 0 {
			continue
		}
		drives = append(drives, Drive{
			Mount:       letter,
			Total:       usage.Total,
			Used:        usage.Used,
			Free:        usage.Free,
			UsedPercent: usage.UsedPercent,
			FSType:      p.Fstype,
		})
	}
	sort.SliceStable(drives, func(i, j int) bool { return drives[i].Mount < drives[j].Mount })
	return drives
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
	if !g.procsAt.IsZero() && time.Since(g.procsAt) < g.procEvery {
		return g.cachedProcs
	}
	live, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return g.cachedProcs
	}
	now := time.Now()
	samples := make([]rawProcSample, 0, len(live))
	handles := make(map[int32]*process.Process, len(live))
	for _, p := range live {
		if ctx.Err() != nil {
			break
		}
		name, err := p.NameWithContext(ctx)
		if err != nil || name == "" {
			continue
		}
		s := rawProcSample{pid: p.Pid, name: name, at: now, handle: p}
		if created, err := p.CreateTimeWithContext(ctx); err == nil && created > 0 {
			if times, err := p.TimesWithContext(ctx); err == nil && times != nil {
				s.created = created
				s.total = times.Total()
				s.hasCPU = true
			}
		}
		if info, err := p.MemoryInfoWithContext(ctx); err == nil && info != nil {
			s.rss = info.RSS
		}
		samples = append(samples, s)
		handles[p.Pid] = p
	}

	lookupUser := func(pid int32) string {
		if h, ok := handles[pid]; ok {
			if user, err := h.UsernameWithContext(ctx); err == nil {
				return user
			}
		}
		return ""
	}
	top, next := computeProcesses(samples, g.prevSamples, g.topN, lookupUser)
	g.prevSamples = next
	g.cachedProcs = top
	g.procsAt = now
	return top
}

// computeProcesses calculates per-process CPU deltas against prev, guarding
// against PID reuse via CreateTime, sorts by CPU then memory, keeps topN, and
// invokes lookupUser only for the retained topN processes.
func computeProcesses(samples []rawProcSample, prev map[int32]procSample, topN int, lookupUser func(int32) string) ([]Process, map[int32]procSample) {
	next := make(map[int32]procSample, len(samples))
	out := make([]Process, 0, len(samples))
	for _, s := range samples {
		cpuPct := 0.0
		if s.hasCPU {
			if old, ok := prev[s.pid]; ok && old.created == s.created {
				elapsed := s.at.Sub(old.at).Seconds()
				if elapsed > 0 && s.total >= old.total {
					cpuPct = 100.0 * (s.total - old.total) / elapsed
				}
			}
			next[s.pid] = procSample{created: s.created, total: s.total, at: s.at}
		}
		out = append(out, Process{
			PID:    s.pid,
			Name:   s.name,
			CPU:    cpuPct,
			Memory: s.rss,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CPU != out[j].CPU {
			return out[i].CPU > out[j].CPU
		}
		return out[i].Memory > out[j].Memory
	})
	if topN > 0 && len(out) > topN {
		out = out[:topN]
	}
	if lookupUser != nil {
		for i := range out {
			out[i].User = lookupUser(out[i].PID)
		}
	}
	return out, next
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
