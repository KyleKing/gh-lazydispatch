package frecency

import "time"

// Store holds frecency history keyed by repository (org/repo).
type Store struct {
	Entries map[string][]HistoryEntry `json:"entries"`
}

// HistoryEntry represents a single workflow run in history.
type HistoryEntry struct {
	Workflow  string            `json:"workflow"`
	Branch    string            `json:"branch"`
	Inputs    map[string]string `json:"inputs"`
	RunCount  int               `json:"run_count"`
	LastRunAt time.Time         `json:"last_run_at"`
}

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{
		Entries: make(map[string][]HistoryEntry),
	}
}
