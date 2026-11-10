//go:build !windows

package apps

import "context"

// EnumeratePackaged lists packaged apps. The Windows build asks PowerShell.
func EnumeratePackaged(context.Context) ([]App, error) { return nil, nil }

// ParsePackaged decodes ConvertTo-Json output.
func ParsePackaged([]byte) ([]App, error) { return nil, nil }
