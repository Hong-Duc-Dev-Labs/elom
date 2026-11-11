//go:build windows

package svc

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SHELLEXECUTEINFOW is 112 bytes on 64-bit Windows and 60 on 32-bit; a wrong
// size makes ShellExecuteExW fail with ERROR_INVALID_PARAMETER.
func TestShellExecuteInfoSize(t *testing.T) {
	want := uintptr(60)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		want = 112
	}
	if got := unsafe.Sizeof(shellExecuteInfo{}); got != want {
		t.Fatalf("sizeof(shellExecuteInfo) = %d, want %d", got, want)
	}
}

func startHandle(t *testing.T, args ...string) (*exec.Cmd, windows.Handle) {
	t.Helper()
	cmd := exec.Command("cmd.exe", args...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { windows.CloseHandle(handle) })
	return cmd, handle
}

func TestWaitProcessReturnsExitCode(t *testing.T) {
	cmd, handle := startHandle(t, "/c", "exit 3")
	defer cmd.Wait()
	code, err := waitProcess(context.Background(), handle)
	if err != nil {
		t.Fatal(err)
	}
	if code != 3 {
		t.Fatalf("exit code %d, want 3", code)
	}
}

func TestWaitProcessStopsOnCancel(t *testing.T) {
	cmd, handle := startHandle(t, "/c", "ping -n 30 127.0.0.1 >nul")
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := waitProcess(ctx, handle)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the context's error", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("cancelling did not stop the wait promptly")
	}
}
