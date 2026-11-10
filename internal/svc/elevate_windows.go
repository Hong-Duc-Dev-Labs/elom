//go:build windows

package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"elom/internal/core/maint"
)

// ErrCancelled means the user dismissed the administrator prompt.
var ErrCancelled = errors.New("elevation: cancelled")

// RunElevated launches the app again with administrator rights, hands it a
// job file and waits for its results. The UI keeps running unelevated.
func RunElevated(ctx context.Context, ops []maint.Op) ([]maint.Result, error) {
	data, err := maint.MarshalJob(ops)
	if err != nil {
		return nil, err
	}
	dir := os.TempDir()
	job, err := os.CreateTemp(dir, "elom-job-*.json")
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
	if err := shellExecuteRunas(exe, `--elevated-job "`+jobPath+`"`); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
		body, err := os.ReadFile(resultPath)
		if err != nil {
			continue
		}
		var results []maint.Result
		if err := json.Unmarshal(body, &results); err != nil {
			return nil, fmt.Errorf("elevation: result: %w", err)
		}
		return results, nil
	}
	return nil, errors.New("elevation: the elevated run did not finish")
}

// shellExecuteRunas starts exe with the runas verb, which raises the UAC
// prompt, and hides its window.
func shellExecuteRunas(exe, params string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	args, err := windows.UTF16PtrFromString(params)
	if err != nil {
		return err
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	shellExecute := shell32.NewProc("ShellExecuteW")
	const swHide = 0
	ret, _, callErr := shellExecute.Call(0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(args)),
		0,
		swHide)
	if ret <= 32 {
		if callErr == windows.ERROR_CANCELLED {
			return ErrCancelled
		}
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("elevation: ShellExecuteW: %w", callErr)
		}
		return fmt.Errorf("elevation: ShellExecuteW failed with code %d", ret)
	}
	return nil
}
