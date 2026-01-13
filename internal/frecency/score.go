package frecency

import (
	"sort"
	"time"
)

// Score calculates the frecency score for an entry.
// Higher scores indicate more frequently and recently used entries.
func Score(entry HistoryEntry) float64 {
	hoursSince := time.Since(entry.LastRunAt).Hours()
	var recency float64
	switch {
	case hoursSince < 1:
		recency = 4.0
	case hoursSince < 24:
		recency = 2.0
	case hoursSince < 168: // 1 week
		recency = 1.0
	default:
		recency = 0.5
	}
	return float64(entry.RunCount) * recency
}

// SortByFrecency sorts entries by frecency score in descending order.
func SortByFrecency(entries []HistoryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return Score(entries[i]) > Score(entries[j])
	})
}

// FilterByWorkflow returns entries matching the given workflow filename.
func FilterByWorkflow(entries []HistoryEntry, workflow string) []HistoryEntry {
	if workflow == "" {
		return entries
	}
	var filtered []HistoryEntry
	for _, e := range entries {
		if e.Workflow == workflow {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
