//go:build windows

package winget

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// List asks winget which installed packages have updates. It returns
// ErrUnparseable when the output is not a table it recognizes.
func List(ctx context.Context) ([]Upgrade, error) {
	out, err := run(ctx, "upgrade", "--include-unknown", "--disable-interactivity", "--accept-source-agreements")
	if err != nil {
		return nil, err
	}
	return ParseUpgrade(out)
}

// Apply updates one package by id, streaming what winget prints.
func Apply(ctx context.Context, id string, onLine func(string)) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("winget: no package id")
	}
	cmd := exec.CommandContext(ctx, "winget.exe",
		"upgrade", "--id", id, "--silent", "--disable-interactivity",
		"--accept-package-agreements", "--accept-source-agreements")
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

// run executes winget and returns its combined output.
func run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "winget.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("winget: %w", err)
	}
	return string(out), nil
}
