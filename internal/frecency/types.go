package frecency

import "time"

// EntryType distinguishes between workflow and chain entries.
type EntryType string

// History entry types.
const (
	EntryTypeWorkflow EntryType = "workflow"
	EntryTypeChain    EntryType = "chain"
)

// Store holds frecency history keyed by repository (org/repo).
type Store struct {
	Entries map[string][]HistoryEntry `json:"entries"`
}

// ChainStepResult represents the result of a single step in a chain run.
type ChainStepResult struct {
	Workflow   string `json:"workflow"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	RunID      int64  `json:"run_id"`
}

// HistoryEntry represents a single workflow or chain run in history.
type HistoryEntry struct {
	LastRunAt   time.Time         `json:"last_run_at"`
	Inputs      map[string]string `json:"inputs"`
	Type        EntryType         `json:"type"`
	Workflow    string            `json:"workflow"`
	ChainName   string            `json:"chain_name,omitempty"`
	Branch      string            `json:"branch"`
	StepResults []ChainStepResult `json:"step_results,omitempty"`
	RunCount    int               `json:"run_count"`
}

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{
		Entries: make(map[string][]HistoryEntry),
	}
}
