// Package svc holds the orchestration the UI calls: it ties the catalog, the
// scanner, the settings and the deletion together, and it is the only place
// that decides whether a destructive run may start (protection list and log
// writable). It knows nothing of the widgets.
package svc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"elom/internal/core/delete"
	"elom/internal/core/oplog"
	"elom/internal/core/rules"
	"elom/internal/core/safety"
	"elom/internal/core/scan"
	"elom/internal/core/settings"
)

// Clean scans the catalog and removes what the user approved.
type Clean struct {
	Guard   *safety.Guard
	Store   *settings.Store
	Log     *oplog.Log
	Catalog []rules.Rule
	Trash   delete.Trasher
}

// NewClean wires a Clean service. catalog may be nil to use the built-in one.
func NewClean(guard *safety.Guard, store *settings.Store, log *oplog.Log, catalog []rules.Rule, trash delete.Trasher) *Clean {
	if catalog == nil {
		catalog = rules.Builtin()
	}
	return &Clean{Guard: guard, Store: store, Log: log, Catalog: catalog, Trash: trash}
}

// Scan expands and sizes the catalog. It refuses to start when the protection
// list or the operation log cannot be read: the user must be able to trust
// that what is listed agrees with what is protected.
func (s *Clean) Scan(ctx context.Context, progress func(scan.Progress)) ([]scan.Candidate, error) {
	if s.Store != nil {
		if _, err := s.Store.Whitelist(); err != nil {
			return nil, err
		}
	}
	if s.Log != nil {
		if err := s.Log.RequireWritable(); err != nil {
			return nil, err
		}
	}
	var whitelist []string
	if s.Store != nil {
		whitelist, _ = s.Store.Whitelist()
	}
	return scan.Run(ctx, s.Guard, s.Catalog, scan.Options{Whitelist: whitelist}, progress), nil
}

// Delete removes the approved candidates. permanent applies only to the
// temporary-file category; see delete.PermanentCategory.
func (s *Clean) Delete(ctx context.Context, candidates []scan.Candidate, permanent bool) []delete.Result {
	items := make([]delete.Item, 0, len(candidates))
	for _, c := range candidates {
		if c.Blocked != nil {
			continue
		}
		items = append(items, delete.Item{Path: c.Path, Bytes: c.Bytes, Category: c.Category})
	}
	return delete.Run(ctx, s.Guard, s.Log, NewRunID(), items, s.Trash, permanent)
}

// DeletePaths removes arbitrary paths, as Analyze's selection: always to the
// Recycle Bin, since they are not temporary files.
func (s *Clean) DeletePaths(ctx context.Context, paths []string) []delete.Result {
	items := make([]delete.Item, 0, len(paths))
	for _, path := range paths {
		items = append(items, delete.Item{Path: path})
	}
	return delete.Run(ctx, s.Guard, s.Log, NewRunID(), items, s.Trash, false)
}

// NewRunID names one destructive run in the operation log.
func NewRunID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "run"
	}
	return fmt.Sprintf("%s-%s", "run", hex.EncodeToString(b[:]))
}
