// Package scan sizes what the cleanup catalog matches: it expands rules,
// checks every path with the safety guard, honours the user's protection
// list, and measures bytes, files and the newest write.
package scan

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"elom/internal/core/rules"
	"elom/internal/core/safety"
)

// Candidate is one path the review screen shows.
type Candidate struct {
	RuleID      string
	Category    string
	App         string
	Note        string
	Path        string
	Bytes       int64
	Files       int
	Modified    time.Time
	Regenerates bool
	// Checked is the initial state of the review's checkbox.
	Checked bool
	// Blocked says why the path is shown but cannot be deleted: a guard
	// refusal or a protection-list match.
	Blocked error
	// Skipped marks a candidate the scan filtered out: too new for its
	// rule, empty, or gone since it was expanded. The UI does not show it.
	Skipped bool
}

// Progress reports how far a scan got.
type Progress struct {
	Done, Total int
	Path        string
}

// Options are the scan's inputs besides the catalog.
type Options struct {
	// Whitelist holds the protection-list patterns.
	Whitelist []string
	// Now is the reference time for the age filter; zero means time.Now.
	Now time.Time
	// Workers is the sizing concurrency; zero means min(NumCPU, 8).
	Workers int
}

// Run expands and sizes the catalog. It returns every candidate, including
// blocked ones, sorted largest first. Cancelling ctx stops the work and
// returns what was measured so far.
func Run(ctx context.Context, guard *safety.Guard, catalog []rules.Rule, opt Options, progress func(Progress)) []Candidate {
	now := opt.Now
	if now.IsZero() {
		now = time.Now()
	}
	workers := opt.Workers
	if workers <= 0 {
		workers = min(runtime.NumCPU(), 8)
	}

	type job struct {
		rule rules.Rule
		path string
	}
	var jobs []job
	seen := make(map[string]int)
	for _, rule := range catalog {
		paths, err := rules.Expand(rule.Paths)
		if err != nil {
			continue // a rule naming a folder this PC does not have
		}
		for _, path := range paths {
			key := strings.ToLower(filepath.Clean(path))
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = len(jobs)
			jobs = append(jobs, job{rule: rule, path: path})
		}
	}

	out := make([]Candidate, len(jobs))
	var mu sync.Mutex
	next := make(chan int)
	var wg sync.WaitGroup
	done := 0
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				select {
				case <-ctx.Done():
					out[i] = Candidate{RuleID: jobs[i].rule.ID, Path: jobs[i].path, Blocked: ctx.Err()}
					continue
				default:
				}
				cand, keep := measure(guard, opt.Whitelist, jobs[i].rule, jobs[i].path, now)
				if !keep {
					cand = Candidate{RuleID: jobs[i].rule.ID, Path: jobs[i].path, Skipped: true}
				}
				out[i] = cand
				mu.Lock()
				done++
				if progress != nil {
					progress(Progress{Done: done, Total: len(jobs), Path: jobs[i].path})
				}
				mu.Unlock()
			}
		}()
	}
	for i := range jobs {
		next <- i
	}
	close(next)
	wg.Wait()

	kept := out[:0]
	for _, c := range out {
		if !c.Skipped {
			kept = append(kept, c)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Bytes > kept[j].Bytes })
	return kept
}

func measure(guard *safety.Guard, whitelist []string, rule rules.Rule, path string, now time.Time) (Candidate, bool) {
	c := Candidate{
		RuleID:      rule.ID,
		Category:    rule.Category,
		App:         rule.App,
		Note:        rule.Note,
		Path:        path,
		Regenerates: rule.Regenerates,
		Checked:     rule.DefaultChecked,
	}
	if err := guard.Check(path); err != nil {
		c.Blocked = err
		return c, true
	}
	if blockedByWhitelist(whitelist, path) {
		c.Blocked = errProtected
		return c, true
	}
	info, err := os.Lstat(path)
	if err != nil {
		return c, false
	}
	c.Modified = info.ModTime()
	if rule.MinAgeDays > 0 && now.Sub(info.ModTime()) < time.Duration(rule.MinAgeDays)*24*time.Hour {
		return c, false
	}
	if info.IsDir() {
		c.Bytes, c.Files, c.Modified = dirSize(path, now)
	} else {
		c.Bytes, c.Files = info.Size(), 1
	}
	if c.Bytes == 0 {
		return c, false
	}
	return c, true
}

// Size sums a directory or file without following links, for the views that
// show how much a leftover holds.
func Size(path string) (bytes int64, files int) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, 0
	}
	if !info.IsDir() {
		return info.Size(), 1
	}
	bytes, files, _ = dirSize(path, time.Now())
	return bytes, files
}

// dirSize sums a directory without following links or leaving it.
func dirSize(root string, now time.Time) (bytes int64, files int, newest time.Time) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && d.Type()&os.ModeSymlink != 0 {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		bytes += info.Size()
		files++
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return bytes, files, newest
}

// blockedByWhitelist matches a pattern against the path itself, or treats a
// pattern without wildcards as a folder whose contents are protected.
func blockedByWhitelist(patterns []string, path string) bool {
	p := strings.ToLower(filepath.Clean(path))
	for _, pattern := range patterns {
		pattern = strings.ToLower(filepath.Clean(pattern))
		if !strings.ContainsAny(pattern, "*?") {
			if p == pattern || strings.HasPrefix(p, pattern+`\`) {
				return true
			}
			continue
		}
		if ok, err := filepath.Match(pattern, p); err == nil && ok {
			return true
		}
	}
	return false
}

var (
	errProtected = errors.New("protected by your list")
)
