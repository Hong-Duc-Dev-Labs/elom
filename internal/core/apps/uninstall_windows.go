//go:build windows

package apps

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"syscall"
)

// RunUninstall runs the app's own uninstaller, streaming its output. The
// command comes from the registry of this machine and is passed to cmd /c as
// the vendor wrote it; Elom never composes one.
func RunUninstall(ctx context.Context, command string, onLine func(string)) error {
	if strings.TrimSpace(command) == "" {
		return ErrNoUninstaller
	}
	cmd := exec.CommandContext(ctx, "cmd.exe", "/c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	read := func(pipe interface{ Read([]byte) (int, error) }) {
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 0, 4096), 64*1024)
		for scanner.Scan() {
			if onLine != nil {
				onLine(scanner.Text())
			}
		}
	}
	done := make(chan struct{}, 2)
	go func() { read(stdout); done <- struct{}{} }()
	go func() { read(stderr); done <- struct{}{} }()
	<-done
	<-done
	return cmd.Wait()
}
