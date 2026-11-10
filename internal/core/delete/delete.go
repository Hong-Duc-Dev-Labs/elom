// Package delete removes what the user approved: to the Recycle Bin by
// default, permanently only for temporary files the user opted into. It
// re-checks the guard at deletion time and writes one log record per path.
package delete

import (
	"context"
	"errors"
	"fmt"
	"os"

	"elom/internal/core/oplog"
	"elom/internal/core/safety"
)

// Item is one approved path.
type Item struct {
	Path     string
	Bytes    int64
	Category string
}

// Result is the outcome for one path.
type Result struct {
	Path    string
	Bytes   int64
	Outcome oplog.Result
	Err     string
}

// Trasher sends a path to the Recycle Bin. The desktop layer implements it
// with MyGo's Shell.TrashItem, which fails rather than deleting for good.
type Trasher interface {
	Trash(path string) error
}

// ErrLogUnwritable stops a run when the operation log cannot be written: a
// deletion without evidence is not allowed.
var ErrLogUnwritable = errors.New("delete: operation log is not writable")

// PermanentCategory is the only category the permanent mode applies to.
const PermanentCategory = "system-temp"

// Run deletes items one by one, never stopping at a per-item failure, and
// records every outcome. When the log cannot be appended to, it stops and
// marks the remaining items as failed.
func Run(ctx context.Context, guard *safety.Guard, log *oplog.Log, runID string, items []Item, trash Trasher, permanent bool) []Result {
	results := make([]Result, len(items))
	writer, err := beginRun(log)
	if err != nil {
		for i, item := range items {
			results[i] = Result{Path: item.Path, Bytes: item.Bytes, Outcome: oplog.ResultFailed, Err: err.Error()}
		}
		return results
	}
	defer writer.Close()
	for i, item := range items {
		res, logBroken := remove(ctx, guard, writer, runID, item, trash, permanent)
		results[i] = res
		if !logBroken {
			continue
		}
		for j := i + 1; j < len(items); j++ {
			results[j] = Result{
				Path:    items[j].Path,
				Bytes:   items[j].Bytes,
				Outcome: oplog.ResultFailed,
				Err:     ErrLogUnwritable.Error(),
			}
		}
		break
	}
	return results
}

// beginRun proves the log is writable once, before the first deletion.
func beginRun(log *oplog.Log) (*oplog.Writer, error) {
	if log == nil {
		return nil, ErrLogUnwritable
	}
	writer, err := log.Run()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLogUnwritable, err)
	}
	return writer, nil
}

func remove(ctx context.Context, guard *safety.Guard, log *oplog.Writer, runID string, item Item, trash Trasher, permanent bool) (Result, bool) {
	res := Result{Path: item.Path, Bytes: item.Bytes, Outcome: oplog.ResultFailed}
	if err := ctx.Err(); err != nil {
		res.Err = "cancelled"
		return res, false
	}
	if err := guard.Check(item.Path); err != nil {
		res.Err = err.Error()
		return res, false
	}

	usePermanent := permanent && item.Category == PermanentCategory
	var err error
	if usePermanent {
		err = os.RemoveAll(item.Path)
		res.Outcome = oplog.ResultPermanent
	} else {
		if trash == nil {
			res.Err = "no Recycle Bin available"
			return res, false
		}
		err = trash.Trash(item.Path)
		res.Outcome = oplog.ResultTrashed
	}
	if err != nil {
		res.Outcome = oplog.ResultFailed
		res.Err = err.Error()
	}

	record := oplog.Record{
		RunID:  runID,
		Op:     "clean",
		Path:   item.Path,
		Bytes:  item.Bytes,
		Result: res.Outcome,
		Err:    res.Err,
	}
	if logErr := log.Append(record); logErr != nil {
		res.Outcome = oplog.ResultFailed
		res.Err = ErrLogUnwritable.Error() + ": " + logErr.Error()
		return res, true
	}
	return res, false
}
