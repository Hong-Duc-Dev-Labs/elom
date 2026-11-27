package ui

import (
	"fmt"
	"testing"
	"time"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/apps"
	"elom/internal/core/metrics"
	"elom/internal/core/oplog"
	"elom/internal/core/scan"
	"elom/internal/core/settings"
)

func newBenchApp(b *testing.B) *App {
	b.Helper()
	dir := b.TempDir()
	store, err := settings.Open(dir)
	if err != nil {
		b.Fatal(err)
	}
	log, err := oplog.Open(dir)
	if err != nil {
		b.Fatal(err)
	}
	return New(dir, store, log)
}

func BenchmarkViewCleanHero(b *testing.B) {
	a := newBenchApp(b)
	a.Screen = "clean"
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = Render(a.View, 1180, 760, 1)
	}
}

func BenchmarkViewStatus40Procs(b *testing.B) {
	a := newBenchApp(b)
	a.Screen = "status"
	snap := sampleSnapshot()
	procs := make([]metrics.Process, 40)
	for i := range procs {
		procs[i] = metrics.Process{
			PID:    int32(1000 + i),
			Name:   fmt.Sprintf("worker-%02d.exe", i),
			CPU:    float64(40-i) * 0.7,
			Memory: uint64(50+i*15) * 1_000_000,
		}
	}
	snap.Processes = procs
	snap.History.CPU = make([]float64, 60)
	snap.History.Disk = make([]float64, 60)
	snap.History.NetDown = make([]float64, 60)
	snap.History.NetUp = make([]float64, 60)
	for i := range 60 {
		snap.History.CPU[i] = float64(10 + i%30)
		snap.History.Disk[i] = float64(100_000 * (i + 1))
		snap.History.NetDown[i] = float64(50_000 * (i + 1))
		snap.History.NetUp[i] = float64(20_000 * (i + 1))
	}
	a.status = snap
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = Render(a.View, 1180, 760, 1)
	}
}

func BenchmarkViewApps100(b *testing.B) {
	a := newBenchApp(b)
	a.Screen = "software"
	a.software.loaded = true
	list := make([]apps.App, 100)
	for i := range list {
		list[i] = apps.App{
			ID:        fmt.Sprintf("app-%03d", i),
			Name:      fmt.Sprintf("Application %03d", i),
			Publisher: "Example Corp",
			Version:   "1.2.3",
			SizeBytes: int64(10+i) * 1_000_000,
		}
	}
	a.software.list = list
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = Render(a.View, 1180, 760, 1)
	}
}

func BenchmarkViewAnalyze200(b *testing.B) {
	a := newBenchApp(b)
	a.Screen = "analyze"
	root := &scan.Node{Path: `C:\Users\ada`, Name: "ada", Dir: true, Bytes: 1_000_000_000, Files: 200}
	root.Children = make([]*scan.Node, 200)
	for i := range 200 {
		root.Children[i] = &scan.Node{
			Path:  fmt.Sprintf(`C:\Users\ada\dir-%03d`, i),
			Name:  fmt.Sprintf("dir-%03d", i),
			Dir:   true,
			Bytes: int64(200-i) * 1_000_000,
			Files: 5,
		}
	}
	a.analyze.root = root
	a.analyze.current = root
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = Render(a.View, 1180, 760, 1)
	}
}

func BenchmarkViewSettings200Logs(b *testing.B) {
	a := newBenchApp(b)
	a.Screen = "settings"
	for i := range 200 {
		_ = a.Log.Append(oplog.Record{
			Time:   time.Now(),
			RunID:  "run-bench",
			Op:     "clean",
			Path:   fmt.Sprintf(`C:\Users\ada\AppData\Local\Temp\item-%03d`, i),
			Bytes:  1024 * int64(i+1),
			Result: oplog.ResultTrashed,
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = Render(a.View, 1180, 760, 1)
	}
}
