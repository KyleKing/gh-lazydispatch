package frecency

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CachePath returns the path to the frecency cache file.
func CachePath() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "gh-wfd", "history.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "gh-wfd", "history.json")
}

// Load reads the store from disk, returning empty store if not found.
func Load() (*Store, error) {
	return LoadFrom(CachePath())
}

// LoadFrom reads the store from a specific path.
func LoadFrom(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewStore(), nil
		}
		return nil, err
	}

	var store Store
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Record adds or updates a history entry for the given repo.
func (s *Store) Record(repo string, workflow, branch string, inputs map[string]string) {
	entries := s.Entries[repo]

	for i, e := range entries {
		if e.Workflow == workflow && e.Branch == branch && mapsEqual(e.Inputs, inputs) {
			entries[i].RunCount++
			entries[i].LastRunAt = time.Now()
			s.Entries[repo] = entries
			return
		}
	}

	entries = append(entries, HistoryEntry{
		Workflow:  workflow,
		Branch:    branch,
		Inputs:    inputs,
		RunCount:  1,
		LastRunAt: time.Now(),
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
