package ui

import (
	"context"
	"strings"
	"sync"
	"testing"

	. "github.com/egoist/mygo/ui"

	"elom/internal/core/maint"
)

func TestOptimizePageRendersTasks(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "optimize"
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"Flush the DNS cache", "Optimize the system drive", "needs administrator", "Each task is a named operation"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
}

// The routine used to set running to "routine", which no row matched, so a
// long defrag ran with no progress anywhere. The row must see its own op.
func TestRoutineMarksTheRunningTask(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "optimize"
	var once sync.Once
	started := make(chan maint.Op, 1)
	release := make(chan struct{})
	a.optimize.run = func(ctx context.Context, op maint.Op) maint.Result {
		once.Do(func() {
			started <- op
			<-release
		})
		return maint.Result{Op: op.Name, OK: true}
	}
	a.Elevate = func(ctx context.Context, ops []maint.Op) ([]maint.Result, error) {
		return nil, ctx.Err()
	}

	a.runRoutine()
	op := <-started
	if a.optimize.running != op.Name {
		t.Fatalf("running = %q, want the task's op %q", a.optimize.running, op.Name)
	}
	if !strings.Contains(a.optimize.note, "Running") {
		t.Errorf("note = %q, want the running task", a.optimize.note)
	}
	if a.optimize.cancel == nil {
		t.Error("no cancel while the routine runs")
	}
	a.optimize.cancel()
	close(release)
}

func TestOptimizePageShowsResult(t *testing.T) {
	a := newTestApp(t)
	a.Screen = "optimize"
	a.optimize.results["dns.flush"] = maint.Result{Op: "dns.flush", OK: true, Detail: "Successfully flushed the DNS Resolver Cache."}
	tt := NewTester(a.View, 1180, 760)
	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "Done: Successfully flushed") {
		t.Errorf("result not shown:\n%s", texts)
	}

	a.optimize.results["cache.icon"] = maint.Result{Op: "cache.icon", Err: "access denied"}
	tt = NewTester(a.View, 1180, 760)
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "Failed: access denied") {
		t.Errorf("failure not shown: %q", tt.Texts())
	}
}
