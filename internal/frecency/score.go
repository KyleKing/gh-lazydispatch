// Package frecency provides frequency and recency-based scoring for workflow history.
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
// Only filters workflow-type entries (chains are excluded).
func FilterByWorkflow(entries []HistoryEntry, workflow string) []HistoryEntry {
	if workflow == "" {
		return entries
	}

	var filtered []HistoryEntry

	for _, e := range entries {
		if e.Workflow == workflow && (e.Type == EntryTypeWorkflow || e.Type == "") {
			filtered = append(filtered, e)
		}
	}

	return filtered
}

// FilterByType returns entries matching the given entry type.
// Empty type "" is treated as workflow for backward compatibility.
func FilterByType(entries []HistoryEntry, entryType EntryType) []HistoryEntry {
	var filtered []HistoryEntry

	for _, e := range entries {
		effectiveType := e.Type
		if effectiveType == "" {
			effectiveType = EntryTypeWorkflow
		}

		if effectiveType == entryType {
			filtered = append(filtered, e)
		}
	}

	return filtered
}
