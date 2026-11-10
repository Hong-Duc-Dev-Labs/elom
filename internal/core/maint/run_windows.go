//go:build windows

package maint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"elom/internal/core/safety"
	"elom/internal/core/startup"
)

// Run performs one operation. It re-validates the op first: the job file is
// untrusted input, even in the elevated process.
func Run(ctx context.Context, op Op) Result {
	if err := Validate(op); err != nil {
		return Result{Op: op.Name, Err: err.Error()}
	}
	task, _ := Find(op.Name)
	if task.Admin && !isElevated() {
		return Result{Op: op.Name, Err: ErrNeedsAdmin.Error()}
	}
	runCtx, cancel := context.WithTimeout(ctx, task.Timeout)
	defer cancel()

	var detail string
	var err error
	switch op.Name {
	case "dns.flush":
		detail, err = runTool(runCtx, "ipconfig.exe", "/flushdns")
	case "cache.icon":
		detail, err = clearExplorerCache("iconcache_*.db")
	case "cache.thumbnails":
		detail, err = clearExplorerCache("thumbcache_*.db")
	case "cache.delivery-optimization":
		detail, err = runTool(runCtx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Delete-DeliveryOptimizationCache -Force")
	case "temp.system-files":
		detail, err = clearSystemTemp()
	case "disk.optimize":
		detail, err = runTool(runCtx, "defrag.exe", os.Getenv("SystemDrive")+"\\", "/O")
	case "update.cache":
		detail, err = clearUpdateCache(runCtx)
	case "update.report":
		detail, err = runTool(runCtx, "cmd.exe", "/c", "sfc /verifyonly")
	case "startup.set-hklm":
		detail, err = setMachineStartup(op.Args["name"], op.Args["enabled"] == "true")
	default:
		err = ErrUnknownOp
	}
	if err != nil {
		return Result{Op: op.Name, Err: err.Error()}
	}
	return Result{Op: op.Name, OK: true, Detail: detail}
}

// isElevated reports whether this process holds an administrator token.
func isElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// RunJob reads a job, runs its operations and writes their results next to it.
// It is the elevated invocation: no window, no sampler, straight out.
func RunJob(jobPath string) error {
	data, err := os.ReadFile(jobPath)
	if err != nil {
		return err
	}
	job, err := ParseJob(data)
	if err != nil {
		return err
	}
	results := make([]Result, 0, len(job.Ops))
	for _, op := range job.Ops {
		results = append(results, Run(context.Background(), op))
	}
	out, err := json.Marshal(results)
	if err != nil {
		return err
	}
	return os.WriteFile(ResultPath(jobPath), out, 0o644)
}

// ResultPath is where an elevated run writes its results.
func ResultPath(jobPath string) string { return jobPath + ".result.json" }

func runTool(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, fmt.Errorf("%s: %w", name, err)
	}
	return firstLine(text), nil
}

func firstLine(text string) string {
	if at := strings.IndexByte(text, '\n'); at >= 0 {
		return strings.TrimSpace(text[:at])
	}
	return text
}

// clearExplorerCache deletes Explorer's cache files of one name pattern. The
// files are inside the user's own profile and are checked by the guard.
func clearExplorerCache(pattern string) (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	dir := filepath.Join(local, `Microsoft\Windows\Explorer`)
	guard := safety.New()
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	removed := 0
	for _, path := range matches {
		if err := guard.Check(path); err != nil {
			continue
		}
		if err := os.Remove(path); err == nil {
			removed++
		}
	}
	return fmt.Sprintf("%d files removed", removed), nil
}

// clearSystemTemp deletes files older than a week in %SystemRoot%\Temp.
func clearSystemTemp() (string, error) {
	root := filepath.Join(os.Getenv("SystemRoot"), "Temp")
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	removed := 0
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			return nil
		}
		if os.Remove(path) == nil {
			removed++
		}
		return nil
	})
	return fmt.Sprintf("%d files removed", removed), nil
}

// clearUpdateCache stops the update services, clears the downloaded updates
// and starts them again, even when a step fails.
func clearUpdateCache(ctx context.Context) (string, error) {
	stop := func(service string) { _ = exec.CommandContext(ctx, "net.exe", "stop", service).Run() }
	start := func(service string) { _ = exec.CommandContext(ctx, "net.exe", "start", service).Run() }
	stop("wuauserv")
	stop("bits")
	defer start("bits")
	defer start("wuauserv")

	dir := filepath.Join(os.Getenv("SystemRoot"), `SoftwareDistribution\Download`)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	removed := 0
	for _, entry := range entries {
		if os.RemoveAll(filepath.Join(dir, entry.Name())) == nil {
			removed++
		}
	}
	return fmt.Sprintf("%d items removed", removed), nil
}

// setMachineStartup writes the StartupApproved flag for an all-users Run
// entry.
func setMachineStartup(name string, enabled bool) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("maint: no startup value name")
	}
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`,
		registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()
	if err := key.SetBinaryValue(name, startup.ApprovedBlob(enabled)); err != nil {
		return "", err
	}
	return "startup flag written", nil
}
