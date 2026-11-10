//go:build !windows

package winget

import (
	"context"
	"errors"
)

// List asks winget which packages have updates. The Windows build runs it.
func List(context.Context) ([]Upgrade, error) { return nil, errors.New("winget: only on Windows") }

// Apply updates one package by id. The Windows build runs it.
func Apply(context.Context, string, func(string)) error {
	return errors.New("winget: only on Windows")
}
