package svc

import (
	"context"
	"time"

	"elom/internal/core/metrics"
)

// Status owns the metrics sampler: its lifecycle and its latest sample.
type Status struct {
	sampler *metrics.Sampler
}

// NewStatus returns a status service reading the machine once a second.
func NewStatus() *Status {
	return NewStatusWith(metrics.NewGopsutilReader(), time.Second)
}

// NewStatusWith takes the reader and interval, for tests.
func NewStatusWith(reader metrics.Reader, interval time.Duration) *Status {
	return &Status{sampler: metrics.NewSampler(reader, interval)}
}

// Start samples until ctx ends or Stop is called. onUpdate runs after every
// sample, on the sampler's goroutine.
func (s *Status) Start(ctx context.Context, onUpdate func(metrics.Snapshot)) {
	s.sampler.Start(ctx, onUpdate)
}

// Stop ends the sampler.
func (s *Status) Stop() { s.sampler.Stop() }

// Snapshot returns the latest sample.
func (s *Status) Snapshot() metrics.Snapshot { return s.sampler.Snapshot() }
