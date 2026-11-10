//go:build !windows

package apps

import "context"

// RunUninstall runs the app's own uninstaller. The Windows build does.
func RunUninstall(context.Context, string, func(string)) error { return ErrNoUninstaller }
