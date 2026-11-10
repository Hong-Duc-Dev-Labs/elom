//go:build !windows

package svc

import (
	"context"
	"errors"

	"elom/internal/core/maint"
)

// ErrCancelled means the user dismissed the administrator prompt.
var ErrCancelled = errors.New("elevation: cancelled")

// RunElevated runs maintenance with administrator rights. The Windows build
// relaunches the app; elsewhere it is unavailable.
func RunElevated(context.Context, []maint.Op) ([]maint.Result, error) {
	return nil, errors.New("elevation: only on Windows")
}
