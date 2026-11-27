//go:build windows

package maint

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

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
	case "store.reset":
		detail, err = runStoreReset(runCtx)
	case "disk.health":
		detail, err = checkDiskHealth(runCtx)
	case "cache.delivery-optimization":
		detail, err = runTool(runCtx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Delete-DeliveryOptimizationCache -Force")
	case "temp.system-files":
		detail, err = clearSystemTemp()
	case "logs.system":
		detail, err = clearSystemLogsAndDumps()
	case "disk.optimize":
		detail, err = runTool(runCtx, "defrag.exe", os.Getenv("SystemDrive")+"\\", "/O")
	case "update.cache":
		detail, err = clearUpdateCache(runCtx)
	case "component.cleanup":
		detail, err = runTool(runCtx, "dism.exe", "/Online", "/Cleanup-Image", "/StartComponentCleanup")
	case "update.report":
		detail, err = runSystemFileReport(runCtx)
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
	return writeResult(ResultPath(jobPath), out)
}

// writeResult writes the results to a temporary file beside path and renames
// it into place, so the waiting UI never reads half a file.
func writeResult(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// ResultPath is where an elevated run writes its results.
func ResultPath(jobPath string) string { return jobPath + ".result.json" }

func runTool(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	summary := summarizeToolOutput(out)
	if err != nil {
		if summary != "" {
			return summary, fmt.Errorf("%s: %s (%w)", name, summary, err)
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return summary, nil
}

// decodeConsoleOutput handles both UTF-8/ASCII and UTF-16LE output (sfc.exe
// writes UTF-16LE to redirected pipes on Windows).
func decodeConsoleOutput(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	// UTF-16LE BOM or embedded NUL bytes from UTF-16LE ASCII text.
	if (len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE) || bytes.IndexByte(raw, 0) >= 0 {
		data := raw
		if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
			data = data[2:]
		}
		u16 := make([]uint16, 0, len(data)/2)
		for i := 0; i+1 < len(data); i += 2 {
			u16 = append(u16, binary.LittleEndian.Uint16(data[i:i+2]))
		}
		return string(utf16.Decode(u16))
	}
	return string(raw)
}

// summarizeToolOutput extracts the final meaningful conclusion line from a
// Windows CLI tool such as sfc.exe, dism.exe or ipconfig.exe.
func summarizeToolOutput(raw []byte) string {
	text := strings.ReplaceAll(decodeConsoleOutput(raw), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var last string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "deployment image servicing") ||
			strings.HasPrefix(lower, "version:") ||
			strings.HasPrefix(lower, "image version:") ||
			strings.HasPrefix(lower, "beginning system scan") ||
			strings.HasPrefix(lower, "beginning verification phase") ||
			strings.HasPrefix(lower, "verification ") && strings.HasSuffix(lower, "complete.") ||
			strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "%]") {
			continue
		}
		last = trimmed
	}
	if last == "" {
		return strings.TrimSpace(text)
	}
	return last
}

// runSystemFileReport runs both SFC (/verifyonly) and DISM (/CheckHealth) in
// report-only mode and returns their conclusion lines.
func runSystemFileReport(ctx context.Context) (string, error) {
	sfcSummary, sfcErr := runTool(ctx, "sfc.exe", "/verifyonly")
	dismSummary, dismErr := runTool(ctx, "dism.exe", "/Online", "/Cleanup-Image", "/CheckHealth")

	parts := make([]string, 0, 2)
	if sfcSummary != "" {
		parts = append(parts, "SFC: "+sfcSummary)
	}
	if dismSummary != "" {
		parts = append(parts, "DISM: "+dismSummary)
	}
	joined := strings.Join(parts, " · ")
	if sfcErr != nil && dismErr != nil {
		return joined, fmt.Errorf("%v; %v", sfcErr, dismErr)
	}
	if sfcErr != nil {
		return joined, sfcErr
	}
	if dismErr != nil {
		return joined, dismErr
	}
	if joined == "" {
		return "completed", nil
	}
	return joined, nil
}

// runStoreReset clears the Microsoft Store cache using wsreset.exe.
func runStoreReset(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "wsreset.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("wsreset.exe: %w", err)
	}
	return "Microsoft Store cache reset", nil
}

type physicalDiskInfo struct {
	FriendlyName string `json:"FriendlyName"`
	MediaType    string `json:"MediaType"`
	HealthStatus string `json:"HealthStatus"`
}

// checkDiskHealth queries Windows storage health for all physical disks.
func checkDiskHealth(ctx context.Context) (string, error) {
	script := "Get-PhysicalDisk | Select-Object FriendlyName,MediaType,HealthStatus | ConvertTo-Json -Compress"
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Get-PhysicalDisk: %w", err)
	}
	return parsePhysicalDisks(out)
}

func parsePhysicalDisks(data []byte) (string, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "no physical disks reported", nil
	}
	var disks []physicalDiskInfo
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &disks); err != nil {
			return "", fmt.Errorf("disk health: %w", err)
		}
	} else {
		var one physicalDiskInfo
		if err := json.Unmarshal(trimmed, &one); err != nil {
			return "", fmt.Errorf("disk health: %w", err)
		}
		disks = []physicalDiskInfo{one}
	}
	parts := make([]string, 0, len(disks))
	for _, d := range disks {
		name := strings.TrimSpace(d.FriendlyName)
		if name == "" {
			name = "Disk"
		}
		status := strings.TrimSpace(d.HealthStatus)
		if status == "" {
			status = "Unknown"
		}
		if media := strings.TrimSpace(d.MediaType); media != "" && !strings.EqualFold(media, "Unspecified") {
			parts = append(parts, fmt.Sprintf("%s (%s): %s", name, media, status))
		} else {
			parts = append(parts, fmt.Sprintf("%s: %s", name, status))
		}
	}
	if len(parts) == 0 {
		return "no physical disks reported", nil
	}
	return strings.Join(parts, " · "), nil
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
	removed := clearOldFilesInDirs([]string{root}, time.Now().Add(-7*24*time.Hour))
	return fmt.Sprintf("%d files removed", removed), nil
}

// clearSystemLogsAndDumps deletes files older than a week in Windows CBS,
// DISM and WindowsUpdate logs plus %SystemRoot%\Minidump.
func clearSystemLogsAndDumps() (string, error) {
	sysRoot := os.Getenv("SystemRoot")
	dirs := []string{
		filepath.Join(sysRoot, `Logs\CBS`),
		filepath.Join(sysRoot, `Logs\DISM`),
		filepath.Join(sysRoot, `Logs\WindowsUpdate`),
		filepath.Join(sysRoot, `Minidump`),
	}
	removed := clearOldFilesInDirs(dirs, time.Now().Add(-7*24*time.Hour))
	return fmt.Sprintf("%d files removed", removed), nil
}

// clearOldFilesInDirs deletes regular files older than cutoff under dirs,
// skipping symlinks and reparse-point directories.
func clearOldFilesInDirs(dirs []string, cutoff time.Time) int {
	removed := 0
	for _, root := range dirs {
		if root == "" {
			continue
		}
		if info, err := os.Lstat(root); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if path != root && entry.Type()&os.ModeSymlink != 0 {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
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
	}
	return removed
}

// serviceRunning reports whether a Windows service is currently RUNNING.
func serviceRunning(ctx context.Context, name string) bool {
	cmd := exec.CommandContext(ctx, "sc.exe", "query", name)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return parseSCQueryRunning(string(out))
}

func parseSCQueryRunning(out string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		upper := strings.ToUpper(strings.TrimSpace(line))
		if strings.HasPrefix(upper, "STATE") && strings.Contains(upper, "RUNNING") {
			return true
		}
	}
	return false
}

// clearUpdateCache stops the update services that were running, clears the
// downloaded updates and restarts only the services that were running before.
func clearUpdateCache(ctx context.Context) (string, error) {
	stop := func(service string) {
		cmd := exec.CommandContext(ctx, "net.exe", "stop", service)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = cmd.Run()
	}
	start := func(service string) {
		cmd := exec.CommandContext(ctx, "net.exe", "start", service)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = cmd.Run()
	}
	wuWasRunning := serviceRunning(ctx, "wuauserv")
	bitsWasRunning := serviceRunning(ctx, "bits")
	if wuWasRunning {
		stop("wuauserv")
		defer start("wuauserv")
	}
	if bitsWasRunning {
		stop("bits")
		defer start("bits")
	}

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
