//go:build windows

package metrics

import "golang.org/x/sys/windows"

// KillProcess ends a process, as Task Manager's End task does. Windows has no
// graceful terminate in this API: the process is ended.
func KillProcess(pid int32) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	return windows.TerminateProcess(handle, 1)
}
