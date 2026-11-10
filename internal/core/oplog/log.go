// Package oplog records every destructive action in an append-only file, so
// the user can answer "what did it delete" later. It fails closed: callers
// must refuse to delete when the log cannot be written.
package oplog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Result is the outcome of one operation.
type Result string

const (
	ResultTrashed   Result = "trashed"
	ResultPermanent Result = "permanent"
	ResultFailed    Result = "failed"
	ResultDone      Result = "done" // a maintenance operation
)

// Record is one line of the log.
type Record struct {
	RunID  string    `json:"run"`
	Time   time.Time `json:"time"`
	Op     string    `json:"op"`
	Path   string    `json:"path"`
	Bytes  int64     `json:"bytes,omitempty"`
	Result Result    `json:"result"`
	Err    string    `json:"error,omitempty"`
}

// ErrCorrupt reports a log line that cannot be decoded. Reading stops there
// and the error says which line, rather than silently skipping history.
var ErrCorrupt = errors.New("oplog: corrupt record")

// Log appends records to one file. It is safe for concurrent use.
type Log struct {
	path string
	mu   sync.Mutex
}

// Open prepares the log at dir/operations.log, creating the directory when
// needed.
func Open(dir string) (*Log, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Log{path: filepath.Join(dir, "operations.log")}, nil
}

// Path returns the log file's path.
func (l *Log) Path() string { return l.path }

// Append writes one record as a JSON line and flushes it, so a crash never
// loses the evidence of a deletion that already happened.
func (l *Log) Append(r Record) error {
	if r.Time.IsZero() {
		r.Time = time.Now()
	}
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return err
	}
	return f.Sync()
}

// Run opens the log once for a run that writes many records, and checks that
// it is writable. Callers must refuse to delete anything when it fails.
func (l *Log) Run() (*Writer, error) {
	if err := l.RequireWritable(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Writer{f: f}, nil
}

// Writer appends records through one open file. Each record is synced before
// Append returns, so a crash cannot hide a deletion that already happened.
type Writer struct {
	mu sync.Mutex
	f  *os.File
}

// Append writes one record and flushes it.
func (w *Writer) Append(r Record) error {
	if r.Time.IsZero() {
		r.Time = time.Now()
	}
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.f.Write(line); err != nil {
		return err
	}
	return w.f.Sync()
}

// Close releases the file.
func (w *Writer) Close() error { return w.f.Close() }

// RequireWritable reports whether the log can be appended to, without
// writing a record. Cleanup calls it before it deletes anything.
func (l *Log) RequireWritable() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

// Read returns every record, oldest first. A corrupt line is an error that
// names the line number.
func Read(path string) ([]Record, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Bytes()
		if len(text) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(text, &r); err != nil {
			return out, fmt.Errorf("%w at line %d: %v", ErrCorrupt, line, err)
		}
		out = append(out, r)
	}
	if err := scanner.Err(); err != nil {
		return out, err
	}
	return out, nil
}
