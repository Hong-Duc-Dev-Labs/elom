package ui

import (
	"sync"
	"time"
)

// uiThrottle limits how often background workers schedule Window.Update calls
// for progress or streaming log lines so fast scans never flood the UI queue.
type uiThrottle struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

func newUIThrottle(interval time.Duration) *uiThrottle {
	return &uiThrottle{interval: interval}
}

func (t *uiThrottle) allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if !t.last.IsZero() && now.Sub(t.last) < t.interval {
		return false
	}
	t.last = now
	return true
}
