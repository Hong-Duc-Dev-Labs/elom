package ui

import (
	"strings"
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
