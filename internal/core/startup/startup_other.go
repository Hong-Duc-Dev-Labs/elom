//go:build !windows

package startup

import "errors"

// ErrNeedsAdmin is returned when a machine-wide entry would be changed.
var ErrNeedsAdmin = errors.New("startup: a machine-wide entry needs administrator rights")

// Enumerate lists startup entries. The Windows build reads the registry.
func Enumerate() ([]Entry, error) { return nil, nil }

// SetEnabled changes one entry. The Windows build writes the flag Explorer
// reads.
func SetEnabled(Entry, bool) error { return ErrNeedsAdmin }
