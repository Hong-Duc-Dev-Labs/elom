// Package startup lists what starts with Windows and turns entries on and
// off the way Task Manager's Startup tab does: through the StartupApproved
// values the shell reads, not by deleting the command.
package startup

import (
	"bytes"
	"encoding/csv"
	"io"
	"strings"
)

// Entry is one startup item.
type Entry struct {
	// Name is the value name under the Run key, or the shortcut's file name.
	Name string
	// Command is what Windows runs.
	Command string
	// Source names where it lives, such as "HKCU Run" or "Startup folder".
	Source string
	// Scope is "user" or "machine".
	Scope string
	// Enabled is the state Task Manager shows.
	Enabled bool
	// Writable is false for entries Elom does not manage, such as shortcuts
	// whose disabled state no StartupApproved value records.
	Writable bool
	// Note explains why an entry is not writable.
	Note string
}

// ApprovedEnabled reports whether a StartupApproved value leaves an entry
// enabled. A missing value means enabled: Windows starts what no flag
// disables.
func ApprovedEnabled(blob []byte) bool {
	if len(blob) == 0 {
		return true
	}
	return blob[0]&1 == 0
}

// ApprovedBlob encodes the StartupApproved value for a state: 12 bytes whose
// first byte is 2 when enabled and 3 when disabled, as Explorer writes them.
func ApprovedBlob(enabled bool) []byte {
	blob := make([]byte, 12)
	if enabled {
		blob[0] = 2
		return blob
	}
	blob[0] = 3
	return blob
}

// SourceScheduledTask is the stable Source identifier for logon scheduled tasks
// listed in read-only audit mode.
const SourceScheduledTask = "Scheduled Task"

// ParseScheduledTasksCSV parses `schtasks /Query /FO CSV /V` output and
// returns non-Microsoft tasks triggered at user logon in read-only audit mode.
func ParseScheduledTasksCSV(data []byte) []Entry {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return nil
	}
	colTaskName, colStatus, colSchedule, colRun := -1, -1, -1, -1
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		switch key {
		case "taskname":
			colTaskName = i
		case "status":
			colStatus = i
		case "schedule type":
			colSchedule = i
		case "task to run":
			colRun = i
		}
	}
	if colTaskName < 0 || colSchedule < 0 || colRun < 0 {
		return nil
	}

	seen := map[string]bool{}
	var out []Entry
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		get := func(idx int) string {
			if idx < 0 || idx >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[idx])
		}
		taskName := get(colTaskName)
		if taskName == "" || strings.EqualFold(taskName, "TaskName") {
			continue
		}
		// Skip built-in Windows system tasks so the audit stays focused on
		// user and third-party logon tasks.
		lowerName := strings.ToLower(taskName)
		if strings.HasPrefix(lowerName, `\microsoft\windows\`) || strings.HasPrefix(lowerName, `microsoft\windows\`) {
			continue
		}
		sched := strings.ToLower(get(colSchedule))
		if !strings.Contains(sched, "logon") && !strings.Contains(sched, "log on") {
			continue
		}
		cmd := get(colRun)
		if cmd == "" || strings.EqualFold(cmd, "N/A") {
			continue
		}
		if seen[lowerName] {
			continue
		}
		seen[lowerName] = true

		displayName := strings.TrimPrefix(taskName, `\`)
		if idx := strings.LastIndex(displayName, `\`); idx >= 0 && idx+1 < len(displayName) {
			displayName = displayName[idx+1:]
		}
		enabled := !strings.EqualFold(get(colStatus), "disabled")
		out = append(out, Entry{
			Name:     displayName,
			Command:  cmd,
			Source:   SourceScheduledTask,
			Scope:    "user",
			Enabled:  enabled,
			Writable: false,
			Note:     "logon scheduled task (read-only audit)",
		})
	}
	return out
}
