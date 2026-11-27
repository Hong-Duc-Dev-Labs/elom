package svc

import (
	"context"

	"elom/internal/core/delete"
	"elom/internal/core/oplog"
	"elom/internal/core/purge"
	"elom/internal/core/safety"
	"elom/internal/core/settings"
)

// Purge orchestrates developer project artifact discovery and safe removal.
type Purge struct {
	Store *settings.Store
	Log   *oplog.Log
	Trash delete.Trasher
}

// NewPurge wires a Purge service.
func NewPurge(store *settings.Store, log *oplog.Log, trash delete.Trasher) *Purge {
	return &Purge{Store: store, Log: log, Trash: trash}
}

// Paths returns the configured project roots, or suggested existing directories
// when none have been saved yet.
func (s *Purge) Paths() ([]string, error) {
	if s.Store != nil {
		saved, err := s.Store.PurgePaths()
		if err != nil {
			return nil, err
		}
		if saved != nil {
			return saved, nil
		}
	}
	return purge.SuggestedPaths(""), nil
}

// SavePaths persists the configured project roots.
func (s *Purge) SavePaths(paths []string) error {
	if s.Store == nil {
		return nil
	}
	return s.Store.SavePurgePaths(paths)
}

// Scan searches roots for build and dependency artifacts. Like Clean.Scan, it
// fails closed when the protection list is unreadable or the operation log is
// not writable.
func (s *Purge) Scan(ctx context.Context, roots []string, progress func(purge.Progress)) ([]purge.Candidate, error) {
	var whitelist []string
	if s.Store != nil {
		wl, err := s.Store.Whitelist()
		if err != nil {
			return nil, err
		}
		whitelist = wl
	}
	if s.Log != nil {
		if err := s.Log.RequireWritable(); err != nil {
			return nil, err
		}
	}
	return purge.Scan(ctx, roots, purge.Options{Whitelist: whitelist}, progress), nil
}

// Delete moves approved project artifacts to the Recycle Bin using the
// specialized ForProjectRoots guard and records each item in the operation log.
// Project artifacts are never permanently deleted.
func (s *Purge) Delete(ctx context.Context, roots []string, candidates []purge.Candidate) []delete.Result {
	guard := safety.ForProjectRoots(roots...)
	items := make([]delete.Item, 0, len(candidates))
	for _, c := range candidates {
		if c.Blocked != nil {
			continue
		}
		items = append(items, delete.Item{
			Path:     c.Path,
			Bytes:    c.Bytes,
			Category: "dev-purge",
		})
	}
	return delete.Run(ctx, guard, s.Log, NewRunID(), items, s.Trash, false)
}
