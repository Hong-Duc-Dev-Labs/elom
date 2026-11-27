//go:build windows

package scan

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// runningProcessNames returns the lower-cased executable names of currently
// running processes via CreateToolhelp32Snapshot (no cgo, ~1ms).
func runningProcessNames() map[string]bool {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snap, &entry); err != nil {
		return nil
	}
	out := make(map[string]bool, 128)
	for {
		name := strings.ToLower(windows.UTF16ToString(entry.ExeFile[:]))
		if name != "" {
			out[name] = true
		}
		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return out
}
