//go:build !windows

package maint

import (
	"context"
	"errors"
)

// Run performs one operation. The Windows build implements them.
func Run(context.Context, Op) Result {
	return Result{Err: "maint: only on Windows"}
}

// RunJob runs a job. The Windows build does.
func RunJob(string) error { return errors.New("maint: only on Windows") }

// ResultPath is where an elevated run writes its results.
func ResultPath(jobPath string) string { return jobPath + ".result.json" }
