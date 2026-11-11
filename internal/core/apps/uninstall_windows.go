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
	cmd := exec.CommandContext(ctx, "cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CmdLine: uninstallCmdLine(command)}
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

// uninstallCmdLine builds cmd's command line by hand. Go's argument escaping
// turns the vendor's quotes into \", which cmd does not understand, so a
// quoted path such as "C:\Program Files\X\unins.exe" /S would not run. With
// /s, cmd strips exactly the outer pair of quotes added here and runs the
// vendor's string unchanged, however many quotes it holds; /d skips AutoRun.
func uninstallCmdLine(command string) string {
	return `cmd.exe /d /s /c "` + command + `"`
}
