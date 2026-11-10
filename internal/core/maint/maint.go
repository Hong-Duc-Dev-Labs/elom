// Package maint is the maintenance Elom performs only when asked: small,
// named operations with a fixed table, validated again in the elevated
// process that performs them.
package maint

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Op is one requested operation.
type Op struct {
	Name string `json:"op"`
	// Args are operation-specific, such as which startup value to change.
	Args map[string]string `json:"args,omitempty"`
}

// Task describes an operation for the UI.
type Task struct {
	Op      string
	Title   string
	Detail  string
	Admin   bool
	Danger  bool
	Timeout time.Duration
}

// Tasks is the closed table of operations Elom can perform. An op not listed
// here is refused, in the UI and in the elevated process alike.
var Tasks = []Task{
	{Op: "dns.flush", Title: "Flush the DNS cache", Detail: "Clears cached lookups. No files are touched; nothing is lost. Windows 11 refuses it without administrator rights.", Admin: true, Timeout: 30 * time.Second},
	{Op: "cache.icon", Title: "Rebuild the icon cache", Detail: "Deletes Explorer's icon cache; it rebuilds as folders open. Explorer is not restarted.", Admin: false, Timeout: 60 * time.Second},
	{Op: "cache.thumbnails", Title: "Rebuild the thumbnail cache", Detail: "Deletes thumbnail pictures; they come back the next time you open a folder.", Admin: false, Timeout: 60 * time.Second},
	{Op: "cache.delivery-optimization", Title: "Clear Delivery Optimization", Detail: "Removes Windows' update-sharing cache. Downloads that used it share again.", Admin: true, Timeout: 120 * time.Second},
	{Op: "temp.system-files", Title: "Clear old system temporary files", Detail: "Deletes files older than 7 days in %SystemRoot%\\Temp.", Admin: true, Timeout: 120 * time.Second},
	{Op: "disk.optimize", Title: "Optimize the system drive", Detail: "Runs Windows' own defrag/TRIM on C:. It can take minutes and must not be interrupted.", Admin: true, Danger: true, Timeout: 30 * time.Minute},
	{Op: "update.cache", Title: "Clear the Windows Update cache", Detail: "Stops the update services, removes downloaded update files, starts the services. Windows downloads them again.", Admin: true, Danger: true, Timeout: 5 * time.Minute},
	{Op: "update.report", Title: "Check the system files", Detail: "Runs SFC and DISM in report-only mode. Nothing is changed; it can take several minutes.", Admin: true, Danger: true, Timeout: 30 * time.Minute},
	{Op: "startup.set-hklm", Title: "Change a machine-wide startup entry", Detail: "Writes the StartupApproved flag for an all-users entry.", Admin: true, Timeout: 30 * time.Second},
}

// ErrUnknownOp is returned for an operation outside the table.
var ErrUnknownOp = errors.New("maint: unknown operation")

// ErrNeedsAdmin is returned when a privileged operation runs unelevated.
var ErrNeedsAdmin = errors.New("maint: needs administrator rights")

// MaxJobSize bounds a job file read from the elevated process.
const MaxJobSize = 64 << 10

// Job is the file the elevated invocation receives.
type Job struct {
	V       int       `json:"v"`
	Created time.Time `json:"created"`
	Ops     []Op      `json:"ops"`
}

// Find returns the task for an op name.
func Find(name string) (Task, bool) {
	for _, task := range Tasks {
		if task.Op == name {
			return task, true
		}
	}
	return Task{}, false
}

// Validate checks an op against the table and the args its operation needs.
func Validate(op Op) error {
	task, ok := Find(op.Name)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownOp, op.Name)
	}
	switch task.Op {
	case "startup.set-hklm":
		if strings.TrimSpace(op.Args["name"]) == "" {
			return fmt.Errorf("maint: %s needs args.name", task.Op)
		}
		switch op.Args["enabled"] {
		case "true", "false":
		default:
			return fmt.Errorf("maint: %s needs args.enabled of true or false", task.Op)
		}
		if strings.TrimSpace(op.Args["scope"]) != "machine" {
			return fmt.Errorf("maint: %s only changes machine-wide entries", task.Op)
		}
	default:
		if len(op.Args) != 0 {
			return fmt.Errorf("maint: %s takes no arguments", task.Op)
		}
	}
	return nil
}

// ParseJob decodes a job file strictly: unknown fields, a wrong version, an
// empty list or an operation outside the table are errors. The file comes from
// an unelevated process and is untrusted input.
func ParseJob(data []byte) (Job, error) {
	if len(data) > MaxJobSize {
		return Job{}, fmt.Errorf("maint: job file is larger than %d bytes", MaxJobSize)
	}
	var job Job
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&job); err != nil {
		return Job{}, fmt.Errorf("maint: job: %w", err)
	}
	if job.V != 1 {
		return Job{}, fmt.Errorf("maint: job version %d, want 1", job.V)
	}
	if len(job.Ops) == 0 {
		return Job{}, errors.New("maint: job has no operations")
	}
	for _, op := range job.Ops {
		if err := Validate(op); err != nil {
			return Job{}, err
		}
	}
	return job, nil
}

// MarshalJob encodes a job for the elevated invocation.
func MarshalJob(ops []Op) ([]byte, error) {
	for _, op := range ops {
		if err := Validate(op); err != nil {
			return nil, err
		}
	}
	return json.Marshal(Job{V: 1, Created: time.Now(), Ops: ops})
}

// Result is the outcome of one operation.
type Result struct {
	Op     string `json:"op"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
	Err    string `json:"error,omitempty"`
}
