//go:build windows

package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"

	"elom/internal/core/maint"
)

// ErrCancelled means the user dismissed the administrator prompt.
var ErrCancelled = errors.New("elevation: cancelled")

// RunElevated launches the app again with administrator rights, hands it a
// job file and waits for that process to exit, then reads its results. The
// UI keeps running unelevated. There is no deadline of its own: every
// operation in the job is bounded by its task's timeout, and waiting on the
// process means a crash or a refused job is reported at once instead of after
// a long poll.
func RunElevated(ctx context.Context, ops []maint.Op) ([]maint.Result, error) {
	data, err := maint.MarshalJob(ops)
	if err != nil {
		return nil, err
	}
	job, err := os.CreateTemp(os.TempDir(), "elom-job-*.json")
	if err != nil {
		return nil, err
	}
	jobPath := job.Name()
	if _, err := job.Write(data); err != nil {
		job.Close()
		os.Remove(jobPath)
		return nil, err
	}
	job.Close()
	defer os.Remove(jobPath)
	resultPath := maint.ResultPath(jobPath)
	defer os.Remove(resultPath)

	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	process, err := shellExecuteRunas(exe, `--elevated-job "`+jobPath+`"`)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(process)

	code, err := waitProcess(ctx, process)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(resultPath)
	if err != nil {
		return nil, fmt.Errorf("elevation: the elevated run ended without results (exit code %d)", code)
	}
	var results []maint.Result
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("elevation: result: %w", err)
	}
	return results, nil
}

// waitProcess waits for the process to exit and returns its exit code. It
// checks ctx between short waits, so cancelling stops the wait; the elevated
// process itself cannot be ended from an unelevated one and finishes alone.
func waitProcess(ctx context.Context, process windows.Handle) (uint32, error) {
	for {
		event, err := windows.WaitForSingleObject(process, 200)
		if err != nil {
			return 0, fmt.Errorf("elevation: wait: %w", err)
		}
		if event == windows.WAIT_OBJECT_0 {
			var code uint32
			if err := windows.GetExitCodeProcess(process, &code); err != nil {
				return 0, fmt.Errorf("elevation: exit code: %w", err)
			}
			return code, nil
		}
		if err := ctx.Err(); err != nil {
			return 0, err
		}
	}
}

// shellExecuteInfo is SHELLEXECUTEINFOW; Go's alignment matches the C layout
// on both 32- and 64-bit Windows.
type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     uintptr
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    uintptr
	dwHotKey     uint32
	hIcon        uintptr
	hProcess     windows.Handle
}

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
)

var procShellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// shellExecuteRunas starts exe with the runas verb, which raises the UAC
// prompt, hides its window and returns a handle to the new process, which the
// caller closes.
func shellExecuteRunas(exe, params string) (windows.Handle, error) {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return 0, err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, err
	}
	args, err := windows.UTF16PtrFromString(params)
	if err != nil {
		return 0, err
	}
	info := shellExecuteInfo{
		fMask:        seeMaskNoCloseProcess | seeMaskNoAsync,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: args,
		nShow:        windows.SW_HIDE,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	ok, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		if errors.Is(callErr, windows.ERROR_CANCELLED) {
			return 0, ErrCancelled
		}
		if callErr != nil && !errors.Is(callErr, windows.ERROR_SUCCESS) {
			return 0, fmt.Errorf("elevation: ShellExecuteExW: %w", callErr)
		}
		return 0, errors.New("elevation: ShellExecuteExW failed")
	}
	if info.hProcess == 0 {
		return 0, errors.New("elevation: ShellExecuteExW returned no process")
	}
	return info.hProcess, nil
}
