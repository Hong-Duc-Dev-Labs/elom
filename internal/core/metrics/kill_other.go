//go:build !windows

package metrics

import (
	"errors"
	"os"
)

// KillProcess ends a process. The Windows build calls TerminateProcess; this
// one is for development machines and tests elsewhere.
func KillProcess(pid int32) error {
	p, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	if p == nil {
		return errors.New("no such process")
	}
	return p.Kill()
}
