// Package runcache keeps finished workflow runs on disk so a timing report
// fetches only the runs created since the last one.
package runcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

const (
	dirPerm  = 0o750
	filePerm = 0o600
)

// StaleAfter is how long a run may stay unfinished before the cache stops
// waiting for it. Runs queued or hung for longer than this exist, and one of
// them would otherwise pin coverage in the past for good.
const StaleAfter = 24 * time.Hour

// Entry holds the finished runs created in [CoveredFrom, CoveredThrough).
// Every run created in that range is in Runs once it has finished or has been
// unfinished for StaleAfter. A younger run in flight is not stored, so it is
// fetched again.
type Entry struct {
	CoveredFrom    time.Time          `json:"covered_from"`
	CoveredThrough time.Time          `json:"covered_through"`
	Runs           []github.RunTiming `json:"runs"`
}

// Range is a half-open span of run creation times.
type Range struct {
	From, To time.Time
}

// Dir returns the cache directory for one repository.
func Dir(repo string) string {
	root := os.Getenv("XDG_CACHE_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = ""
		}

		root = filepath.Join(home, ".cache")
	}

	return filepath.Join(root, "lazydispatch", filepath.FromSlash(repo))
}

// Path returns the cache file for one repository and run filter. An empty
// branch or actor reads as "all".
func Path(repo string, filter github.RunFilter) string {
	name := "ci-timing-" + sanitize(filter.Event) + "-" + sanitize(orAll(filter.Branch))
	if filter.Actor != "" {
		name += "-" + sanitize(filter.Actor)
	}

	return filepath.Join(Dir(repo), name+".json")
}

func orAll(s string) string {
	if s == "" {
		return "all"
	}

	return s
}

func sanitize(s string) string {
	return strings.NewReplacer("/", "_", string(filepath.Separator), "_").Replace(s)
}

// Load reads the cache at path. A missing file is an empty Entry.
func Load(path string) (Entry, error) {
	var entry Entry

	err := readJSON(path, &entry, "run cache")

	return entry, err
}

// Save writes entry to path through a temporary file, so an interrupted write
// leaves the previous cache intact.
func Save(path string, entry Entry) error {
	return writeJSON(path, entry, "run cache")
}

// readJSON decodes the file at path into v, leaving v as it was when the file
// does not exist.
func readJSON(path string, v any, what string) error {
	data, err := os.ReadFile(path) //nolint:gosec // path derived from XDG_CACHE_HOME/home dir and the repo name
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("reading the %s: %w", what, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parsing the %s %s (delete it to rebuild): %w", what, path, err)
	}

	return nil
}

func writeJSON(path string, v any, what string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("creating the cache directory: %w", err)
	}

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encoding the %s: %w", what, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, filePerm); err != nil {
		return fmt.Errorf("writing the %s: %w", what, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replacing the %s: %w", what, err)
	}

	return nil
}

// Missing returns the ranges within [from, to) that entry does not cover.
func (e Entry) Missing(from, to time.Time) []Range {
	if e.CoveredThrough.IsZero() || e.CoveredFrom.After(to) || e.CoveredThrough.Before(from) {
		return []Range{{From: from, To: to}}
	}

	var missing []Range

	if from.Before(e.CoveredFrom) {
		missing = append(missing, Range{From: from, To: e.CoveredFrom})
	}

	if to.After(e.CoveredThrough) {
		missing = append(missing, Range{From: e.CoveredThrough, To: to})
	}

	return missing
}

// Merge returns entry extended with fetched runs, which must cover every
// Missing range of [from, to). Coverage ends where the earliest in-flight
// fetched run younger than StaleAfter begins, because it must be read again.
// A stale unfinished run is stored with its status, so it keeps its commit out
// of timing reports until the cache is rebuilt.
func (e Entry) Merge(fetched []github.RunTiming, from, to time.Time) Entry {
	coveredFrom, coveredThrough := from, to
	if !e.CoveredThrough.IsZero() {
		if e.CoveredFrom.Before(coveredFrom) {
			coveredFrom = e.CoveredFrom
		}

		if e.CoveredThrough.After(coveredThrough) {
			coveredThrough = e.CoveredThrough
		}
	}

	byID := make(map[int64]github.RunTiming, len(e.Runs)+len(fetched))
	for i := range e.Runs {
		byID[e.Runs[i].ID] = e.Runs[i]
	}

	for i := range fetched {
		run := fetched[i]
		if github.IsComplete(&run) || run.CreatedAt.Before(to.Add(-StaleAfter)) {
			byID[run.ID] = run

			continue
		}

		if run.CreatedAt.Before(coveredThrough) {
			coveredThrough = run.CreatedAt
		}
	}

	if coveredThrough.Before(coveredFrom) {
		coveredThrough = coveredFrom
	}

	merged := Entry{CoveredFrom: coveredFrom, CoveredThrough: coveredThrough}
	for id := range byID {
		merged.Runs = append(merged.Runs, byID[id])
	}

	sort.Slice(merged.Runs, func(i, j int) bool { return merged.Runs[i].ID < merged.Runs[j].ID })

	return merged
}

// Within returns the runs created in [from, to), including fetched runs that
// are still in flight.
func Within(stored, fetched []github.RunTiming, from, to time.Time) []github.RunTiming {
	seen := make(map[int64]bool, len(stored)+len(fetched))

	var out []github.RunTiming

	for _, group := range [][]github.RunTiming{fetched, stored} {
		for i := range group {
			run := &group[i]
			if seen[run.ID] || run.CreatedAt.Before(from) || !run.CreatedAt.Before(to) {
				continue
			}

			seen[run.ID] = true
			out = append(out, *run)
		}
	}

	return out
}
