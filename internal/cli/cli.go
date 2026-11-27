// Package cli implements Elom's read-only and dry-run command-line interface
// for scripts and AI agents. It uses internal/core directly with no UI
// dependency and never performs destructive operations.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"elom/internal/core/metrics"
	"elom/internal/core/oplog"
	"elom/internal/core/rules"
	"elom/internal/core/safety"
	"elom/internal/core/scan"
	"elom/internal/core/settings"
)

// Deps injects external dependencies so CLI commands can be tested without
// touching the live machine or user profile.
type Deps struct {
	DataDir        string
	StatusReader   metrics.Reader
	SampleInterval time.Duration
	Catalog        []rules.Rule
	Guard          *safety.Guard
	ScanOptions    scan.Options
}

// IsCommand reports whether args begins with a recognized CLI subcommand or
// help flag. GUI launches (no arguments or unrecognized flags) return false.
func IsCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch strings.ToLower(args[0]) {
	case "status", "analyze", "history", "clean", "help", "-h", "--help":
		return true
	default:
		return false
	}
}

// Run executes a CLI subcommand using the default machine dependencies.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return RunWithDeps(ctx, args, stdout, stderr, Deps{})
}

// RunWithDeps executes a CLI subcommand with injectable dependencies and
// returns a process exit code.
func RunWithDeps(ctx context.Context, args []string, stdout, stderr io.Writer, deps Deps) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	cmd := strings.ToLower(args[0])
	rest := args[1:]
	var err error
	switch cmd {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "status":
		err = runStatus(ctx, rest, stdout, stderr, deps)
	case "analyze":
		err = runAnalyze(ctx, rest, stdout, stderr, deps)
	case "history":
		err = runHistory(rest, stdout, stderr, deps)
	case "clean":
		err = runClean(ctx, rest, stdout, stderr, deps)
	default:
		fmt.Fprintf(stderr, "elom: unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "elom %s: %v\n", cmd, err)
		var usageErr *usageError
		if errors.As(err, &usageErr) {
			return 2
		}
		return 1
	}
	return 0
}

type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Elom read-only CLI (no destructive actions):")
	fmt.Fprintln(w, "  Elom.exe status [--json]")
	fmt.Fprintln(w, "  Elom.exe analyze [--json] [--depth N] [--entries N] <path>")
	fmt.Fprintln(w, "  Elom.exe history [--json] [--limit N] [--op clean|optimize|uninstall]")
	fmt.Fprintln(w, "  Elom.exe clean --dry-run [--json]")
}

func defaultDataDir(override string) string {
	if override != "" {
		return override
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "Elom")
	}
	return "."
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// --- status ---

type statusOutput struct {
	At            time.Time         `json:"at"`
	CPU           float64           `json:"cpu"`
	MemUsed       uint64            `json:"mem_used"`
	MemTotal      uint64            `json:"mem_total"`
	SwapUsed      uint64            `json:"swap_used"`
	SwapTotal     uint64            `json:"swap_total"`
	DiskReadBps   float64           `json:"disk_read_bps"`
	DiskWriteBps  float64           `json:"disk_write_bps"`
	NetRecvBps    float64           `json:"net_recv_bps"`
	NetSentBps    float64           `json:"net_sent_bps"`
	UptimeSeconds int64             `json:"uptime_seconds"`
	Health        int               `json:"health"`
	Diagnosis     metrics.Diagnosis `json:"diagnosis"`
	SysFree       uint64            `json:"sys_free"`
	SysTotal      uint64            `json:"sys_total"`
	Drives        []metrics.Drive   `json:"drives,omitempty"`
	Battery       *metrics.Battery  `json:"battery,omitempty"`
	System        metrics.System    `json:"system"`
	Cores         []float64         `json:"cores,omitempty"`
	Processes     []metrics.Process `json:"processes,omitempty"`
}

func runStatus(ctx context.Context, args []string, stdout, stderr io.Writer, deps Deps) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return &usageError{msg: fmt.Sprintf("unexpected argument %q", fs.Arg(0))}
	}

	reader := deps.StatusReader
	if reader == nil {
		reader = metrics.NewGopsutilReader()
	}
	interval := deps.SampleInterval
	if interval == 0 {
		interval = 250 * time.Millisecond
	}

	sampler := metrics.NewSampler(reader, interval)
	sampleCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Collect one tick (or two when interval > 0 so cumulative rates are populated).
	done := make(chan metrics.Snapshot, 1)
	if interval < 0 {
		raw, err := reader.Read(ctx)
		if err != nil {
			return err
		}
		done <- metrics.Snapshot{
			At:        time.Now(),
			CPU:       raw.CPUPercent,
			MemUsed:   raw.MemUsed,
			MemTotal:  raw.MemTotal,
			SwapUsed:  raw.SwapUsed,
			SwapTotal: raw.SwapTotal,
			Uptime:    raw.Uptime,
			Processes: raw.Processes,
			SysFree:   raw.SysFree,
			SysTotal:  raw.SysTotal,
			Drives:    raw.Drives,
			Battery:   raw.Battery,
			System:    raw.System,
			Cores:     raw.Cores,
		}
	} else {
		go func() {
			sampler.Start(sampleCtx, func(s metrics.Snapshot) {
				select {
				case done <- s:
				default:
				}
				cancel()
			})
		}()
	}

	var snap metrics.Snapshot
	select {
	case snap = <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	if snap.At.IsZero() {
		return errors.New("failed to read system metrics")
	}

	out := statusOutput{
		At:            snap.At,
		CPU:           snap.CPU,
		MemUsed:       snap.MemUsed,
		MemTotal:      snap.MemTotal,
		SwapUsed:      snap.SwapUsed,
		SwapTotal:     snap.SwapTotal,
		DiskReadBps:   snap.DiskRead,
		DiskWriteBps:  snap.DiskWrite,
		NetRecvBps:    snap.NetRecv,
		NetSentBps:    snap.NetSent,
		UptimeSeconds: int64(snap.Uptime.Seconds()),
		Health:        snap.Health,
		Diagnosis:     snap.Diagnosis,
		SysFree:       snap.SysFree,
		SysTotal:      snap.SysTotal,
		Drives:        snap.Drives,
		Battery:       snap.Battery,
		System:        snap.System,
		Cores:         snap.Cores,
		Processes:     snap.Processes,
	}
	if *asJSON {
		return writeJSON(stdout, out)
	}
	fmt.Fprintf(stdout, "Health: %d/100 (%s)\nCPU: %.1f%%  Memory: %d / %d bytes  Processes: %d\n",
		out.Health, out.Diagnosis.Kind, out.CPU, out.MemUsed, out.MemTotal, len(out.Processes))
	return nil
}

// --- analyze ---

type analyzeNode struct {
	Path           string               `json:"path"`
	Name           string               `json:"name"`
	Dir            bool                 `json:"dir"`
	Bytes          int64                `json:"bytes"`
	Files          int                  `json:"files"`
	Partial        bool                 `json:"partial,omitempty"`
	PartialReasons []scan.PartialReason `json:"partial_reasons,omitempty"`
	Cleanable      bool                 `json:"cleanable,omitempty"`
	Err            string               `json:"err,omitempty"`
	Children       []*analyzeNode       `json:"children,omitempty"`
}

func toAnalyzeNode(n *scan.Node) *analyzeNode {
	if n == nil {
		return nil
	}
	out := &analyzeNode{
		Path:           n.Path,
		Name:           n.Name,
		Dir:            n.Dir,
		Bytes:          n.Bytes,
		Files:          n.Files,
		Partial:        n.Partial,
		PartialReasons: n.PartialReasons,
		Cleanable:      n.Cleanable,
		Err:            n.Err,
	}
	if len(n.Children) > 0 {
		out.Children = make([]*analyzeNode, 0, len(n.Children))
		for _, ch := range n.Children {
			out.Children = append(out.Children, toAnalyzeNode(ch))
		}
	}
	return out
}

func runAnalyze(ctx context.Context, args []string, stdout, stderr io.Writer, _ Deps) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "output JSON")
	maxDepth := fs.Int("depth", settings.DefaultMaxDepth, "maximum directory depth")
	maxEntries := fs.Int("entries", settings.DefaultMaxEntries, "maximum entries to index")

	// Allow flags after the positional path (e.g. `analyze C:\foo --json`).
	reordered := reorderFlags(args, map[string]bool{"depth": true, "entries": true})
	if err := fs.Parse(reordered); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return &usageError{msg: "usage: Elom.exe analyze [--json] [--depth N] [--entries N] <path>"}
	}
	target := fs.Arg(0)
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", target)
	}

	root, err := scan.Index(ctx, target, scan.Limits{
		MaxDepth:   *maxDepth,
		MaxEntries: *maxEntries,
	}, nil)
	if err != nil && !errors.Is(err, scan.ErrLimit) {
		return err
	}
	if *asJSON {
		return writeJSON(stdout, toAnalyzeNode(root))
	}
	fmt.Fprintf(stdout, "%s  %d bytes  (%d files)\n", root.Path, root.Bytes, root.Files)
	for i, ch := range root.Children {
		if i >= 10 {
			break
		}
		fmt.Fprintf(stdout, "  %-40s %12d bytes\n", ch.Name, ch.Bytes)
	}
	return nil
}

// --- history ---

type historyOutput struct {
	Path           string         `json:"path"`
	TotalRecords   int            `json:"total_records"`
	ReclaimedBytes int64          `json:"reclaimed_bytes"`
	Records        []oplog.Record `json:"records"`
}

func runHistory(args []string, stdout, stderr io.Writer, deps Deps) error {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "output JSON")
	limit := fs.Int("limit", 50, "maximum number of most recent records to return (0 for all)")
	opFilter := fs.String("op", "", "filter by operation type (clean, optimize, uninstall)")
	dataDirFlag := fs.String("data-dir", "", "override Elom data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return &usageError{msg: fmt.Sprintf("unexpected argument %q", fs.Arg(0))}
	}
	if *limit < 0 {
		return &usageError{msg: "--limit must be >= 0"}
	}

	dir := defaultDataDir(deps.DataDir)
	if *dataDirFlag != "" {
		dir = *dataDirFlag
	}
	logPath := filepath.Join(dir, "operations.log")
	records, err := oplog.Read(logPath)
	if err != nil && len(records) == 0 {
		return err
	}

	filtered := oplog.Filter(records, *opFilter)
	summary := oplog.Summarize(filtered)
	if *limit > 0 && len(filtered) > *limit {
		filtered = filtered[len(filtered)-*limit:]
	}
	if filtered == nil {
		filtered = []oplog.Record{}
	}

	out := historyOutput{
		Path:           logPath,
		TotalRecords:   summary.TotalRecords,
		ReclaimedBytes: summary.ReclaimedBytes,
		Records:        filtered,
	}
	if *asJSON {
		return writeJSON(stdout, out)
	}
	fmt.Fprintf(stdout, "History: %d records (%d bytes reclaimed)\n", out.TotalRecords, out.ReclaimedBytes)
	for _, r := range out.Records {
		fmt.Fprintf(stdout, "%s  %-10s %-10s %s\n", r.Time.Format(time.RFC3339), r.Op, r.Result, r.Path)
	}
	return nil
}

// --- clean ---

type cleanCandidate struct {
	RuleID         string `json:"rule_id"`
	Category       string `json:"category"`
	App            string `json:"app"`
	Path           string `json:"path"`
	Bytes          int64  `json:"bytes"`
	Files          int    `json:"files"`
	Checked        bool   `json:"checked"`
	RunningProcess string `json:"running_process,omitempty"`
	Blocked        string `json:"blocked,omitempty"`
}

type cleanOutput struct {
	DryRun         bool             `json:"dry_run"`
	TotalBytes     int64            `json:"total_bytes"`
	CandidateCount int              `json:"candidate_count"`
	BlockedCount   int              `json:"blocked_count"`
	Candidates     []cleanCandidate `json:"candidates"`
}

func runClean(ctx context.Context, args []string, stdout, stderr io.Writer, deps Deps) error {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "preview cleanable items without deleting (required)")
	asJSON := fs.Bool("json", false, "output JSON")
	dataDirFlag := fs.String("data-dir", "", "override Elom data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return &usageError{msg: fmt.Sprintf("unexpected argument %q", fs.Arg(0))}
	}
	if !*dryRun {
		return &usageError{msg: "CLI clean only supports --dry-run; use the Elom app to review and delete files"}
	}

	opt := deps.ScanOptions
	if len(opt.Whitelist) == 0 {
		dir := defaultDataDir(deps.DataDir)
		if *dataDirFlag != "" {
			dir = *dataDirFlag
		}
		if store, err := settings.Open(dir); err == nil {
			wl, werr := store.Whitelist()
			if werr != nil {
				return werr
			}
			opt.Whitelist = wl
		}
	}

	guard := deps.Guard
	if guard == nil {
		guard = safety.New()
	}
	catalog := deps.Catalog
	if catalog == nil {
		catalog = rules.Builtin()
	}

	candidates := scan.Run(ctx, guard, catalog, opt, nil)
	out := cleanOutput{
		DryRun:     true,
		Candidates: make([]cleanCandidate, 0, len(candidates)),
	}
	for _, c := range candidates {
		item := cleanCandidate{
			RuleID:         c.RuleID,
			Category:       c.Category,
			App:            c.App,
			Path:           c.Path,
			Bytes:          c.Bytes,
			Files:          c.Files,
			Checked:        c.Checked,
			RunningProcess: c.RunningProcess,
		}
		if c.Blocked != nil {
			item.Blocked = c.Blocked.Error()
			out.BlockedCount++
		} else {
			out.TotalBytes += c.Bytes
		}
		out.Candidates = append(out.Candidates, item)
	}
	out.CandidateCount = len(out.Candidates)

	if *asJSON {
		return writeJSON(stdout, out)
	}
	fmt.Fprintf(stdout, "Dry-run: %d candidates (%d blocked), %d bytes cleanable\n",
		out.CandidateCount, out.BlockedCount, out.TotalBytes)
	for _, c := range out.Candidates {
		fmt.Fprintf(stdout, "  %-24s %12d bytes  %s\n", c.App, c.Bytes, c.Path)
	}
	return nil
}

// reorderFlags moves flag tokens before positional arguments so callers can
// pass flags either before or after positional arguments (e.g. `analyze C:\tmp --json`).
func reorderFlags(args []string, valueFlags map[string]bool) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if !strings.Contains(name, "=") && valueFlags[name] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	return append(flags, pos...)
}
