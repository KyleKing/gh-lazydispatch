package frecency

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	dirPerm  = 0o750
	filePerm = 0o600
)

// CachePath returns the path to the frecency cache file.
// Migrates from old gh-wfd directory to lazydispatch if needed.
func CachePath() string {
	var newPath, oldPath string

	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		newPath = filepath.Join(xdg, "lazydispatch", "history.json")
		oldPath = filepath.Join(xdg, "gh-wfd", "history.json")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			home = ""
		}
		newPath = filepath.Join(home, ".cache", "lazydispatch", "history.json")
		oldPath = filepath.Join(home, ".cache", "gh-wfd", "history.json")
	}

	migrateOldCache(oldPath, newPath)

	return newPath
}

// migrateOldCache best-effort copies the cache file from oldPath to newPath if
// oldPath exists and newPath does not. Both paths are derived internally from
// XDG_CACHE_HOME/the user's home directory, never from external input.
func migrateOldCache(oldPath, newPath string) {
	// #nosec G703 -- path derived from XDG_CACHE_HOME/home dir, not external input
	if _, err := os.Stat(oldPath); err != nil {
		return
	}

	// #nosec G703 -- path derived from XDG_CACHE_HOME/home dir, not external input
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		return
	}

	// #nosec G703 -- path derived from XDG_CACHE_HOME/home dir, not external input
	if err := os.MkdirAll(filepath.Dir(newPath), dirPerm); err != nil {
		return
	}

	data, err := os.ReadFile(oldPath) //nolint:gosec // path derived from XDG_CACHE_HOME/home dir, not external input
	if err != nil {
		return
	}

	// #nosec G703 -- path derived from XDG_CACHE_HOME/home dir, not external input
	//nolint:errcheck // best-effort cache migration; no error return to propagate failure to
	_ = os.WriteFile(newPath, data, filePerm)
}

// Load reads the store from disk, returning empty store if not found.
func Load() (*Store, error) {
	return LoadFrom(CachePath())
}

// LoadFrom reads the store from a specific path.
func LoadFrom(path string) (*Store, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path derived from CachePath (XDG cache dir), not external input
	if err != nil {
		if os.IsNotExist(err) {
			return NewStore(), nil
		}

		return nil, fmt.Errorf("reading frecency store %s: %w", path, err)
	}

	var store Store
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parsing frecency store %s: %w", path, err)
	}

	if store.Entries == nil {
		store.Entries = make(map[string][]HistoryEntry)
	}

	return &store, nil
}

// Save writes the store to disk.
func (s *Store) Save() error {
	return s.SaveTo(CachePath())
}

// SaveTo writes the store to a specific path.
func (s *Store) SaveTo(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("creating directory for frecency store %s: %w", path, err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling frecency store: %w", err)
	}

	if err := os.WriteFile(path, data, filePerm); err != nil {
		return fmt.Errorf("writing frecency store %s: %w", path, err)
	}

	return nil
}

// Record adds or updates a workflow history entry for the given repo.
func (s *Store) Record(repo, workflow, branch string, inputs map[string]string) {
	entries := s.Entries[repo]

	for i := range entries {
		e := &entries[i]
		if e.Type == EntryTypeWorkflow && e.Workflow == workflow && e.Branch == branch && mapsEqual(e.Inputs, inputs) {
			entries[i].RunCount++
			entries[i].LastRunAt = time.Now()
			s.Entries[repo] = entries

			return
		}
	}

	entries = append(entries, HistoryEntry{
		Type:      EntryTypeWorkflow,
		Workflow:  workflow,
		Branch:    branch,
		Inputs:    inputs,
		RunCount:  1,
		LastRunAt: time.Now(),
	})
	s.Entries[repo] = entries
}

// RecordChain adds or updates a chain history entry for the given repo.
func (s *Store) RecordChain(repo, chainName, branch string, inputs map[string]string, stepResults []ChainStepResult) {
	entries := s.Entries[repo]

	for i := range entries {
		e := &entries[i]
		if e.Type != EntryTypeChain || e.ChainName != chainName || e.Branch != branch || !mapsEqual(e.Inputs, inputs) {
			continue
		}

		entries[i].RunCount++
		entries[i].LastRunAt = time.Now()
		entries[i].StepResults = stepResults
		s.Entries[repo] = entries

		return
	}

	entries = append(entries, HistoryEntry{
		Type:        EntryTypeChain,
		ChainName:   chainName,
		Branch:      branch,
		Inputs:      inputs,
		StepResults: stepResults,
		RunCount:    1,
		LastRunAt:   time.Now(),
	})
	s.Entries[repo] = entries
}

// TopForRepo returns the top entries for a repo, optionally filtered by workflow.
func (s *Store) TopForRepo(repo, workflowFilter string, limit int) []HistoryEntry {
	entries := s.Entries[repo]
	if len(entries) == 0 {
		return nil
	}

	result := make([]HistoryEntry, len(entries))
	copy(result, entries)

	result = FilterByWorkflow(result, workflowFilter)
	SortByFrecency(result)

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if b[k] != v {
			return false
		}
	}

	return true
}
