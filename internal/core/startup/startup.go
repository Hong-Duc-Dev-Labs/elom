// Package startup lists what starts with Windows and turns entries on and
// off the way Task Manager's Startup tab does: through the StartupApproved
// values the shell reads, not by deleting the command.
package startup

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
