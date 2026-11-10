//go:build !windows

package apps

// Enumerate lists installed programs. The Windows build reads the registry;
// this one exists so the package builds on development machines elsewhere.
func Enumerate() ([]App, error) { return nil, nil }
