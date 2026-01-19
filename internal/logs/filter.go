package logs

import (
	"regexp"
	"strings"
)

// Filter applies filtering logic to log entries.
type Filter struct {
	config *FilterConfig
	regex  *regexp.Regexp
}

// NewFilter creates a new log filter with the given configuration.
func NewFilter(config *FilterConfig) (*Filter, error) {
	f := &Filter{config: config}

	// Compile regex if needed
	if config.Regex && config.SearchTerm != "" {
		pattern := config.SearchTerm
		if !config.CaseSensitive {
			pattern = "(?i)" + pattern
		}

		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		f.regex = regex
	}

	return f, nil
}

// Apply filters a RunLogs instance and returns filtered entries.
func (f *Filter) Apply(runLogs *RunLogs) *FilteredResult {
	result := &FilteredResult{
		Steps:  make([]*FilteredStepLogs, 0),
		Config: f.config,
	}

	steps := runLogs.AllSteps()
	for _, step := range steps {
		// Skip if filtering by specific step
		if f.config.StepIndex >= 0 && step.StepIndex != f.config.StepIndex {
			continue
		}

		filteredStep := &FilteredStepLogs{
			StepIndex: step.StepIndex,
			Workflow:  step.Workflow,
			StepName:  step.StepName,
			Entries:   make([]FilteredLogEntry, 0),
		}

		for i, entry := range step.Entries {
			if f.matchesEntry(&entry) {
				filteredEntry := FilteredLogEntry{
					Original:      entry,
					OriginalIndex: i,
					Matches:       f.findMatches(entry.Content),
				}
				filteredStep.Entries = append(filteredStep.Entries, filteredEntry)
			}
		}

		// Only include steps that have matching entries
		if len(filteredStep.Entries) > 0 {
			result.Steps = append(result.Steps, filteredStep)
		}
	}

	return result
}

// matchesEntry determines if a log entry matches the filter criteria.
func (f *Filter) matchesEntry(entry *LogEntry) bool {
	// Filter by level
	switch f.config.Level {
	case FilterErrors:
		if entry.Level != LogLevelError {
			return false
		}
	case FilterWarnings:
		if entry.Level != LogLevelWarning && entry.Level != LogLevelError {
			return false
		}
	case FilterAll:
		// No level filtering
	}

	// Filter by search term
	if f.config.SearchTerm == "" {
		return true
	}

	if f.config.Regex {
		return f.regex.MatchString(entry.Content)
	}

	content := entry.Content
	searchTerm := f.config.SearchTerm

	if !f.config.CaseSensitive {
		content = strings.ToLower(content)
		searchTerm = strings.ToLower(searchTerm)
	}

	return strings.Contains(content, searchTerm)
}

// findMatches finds all match positions in the content for highlighting.
func (f *Filter) findMatches(content string) []MatchPosition {
	if f.config.SearchTerm == "" {
		return nil
	}

	var matches []MatchPosition

	if f.config.Regex {
		if f.regex == nil {
			return nil
		}

		indices := f.regex.FindAllStringIndex(content, -1)
		for _, idx := range indices {
			matches = append(matches, MatchPosition{
				Start: idx[0],
				End:   idx[1],
			})
		}
	} else {
		searchTerm := f.config.SearchTerm
		searchContent := content

		if !f.config.CaseSensitive {
			searchTerm = strings.ToLower(searchTerm)
			searchContent = strings.ToLower(searchContent)
		}

		start := 0
		for {
			idx := strings.Index(searchContent[start:], searchTerm)
			if idx == -1 {
				break
			}

			absIdx := start + idx
			matches = append(matches, MatchPosition{
				Start: absIdx,
				End:   absIdx + len(searchTerm),
			})

			start = absIdx + 1
		}
	}

	return matches
}

// FilteredResult contains the filtered logs with match information.
type FilteredResult struct {
	Steps  []*FilteredStepLogs
	Config *FilterConfig
}

// TotalEntries returns the total number of filtered entries.
func (fr *FilteredResult) TotalEntries() int {
	total := 0
	for _, step := range fr.Steps {
		total += len(step.Entries)
	}
	return total
}

// FilteredStepLogs contains filtered logs for a single step.
type FilteredStepLogs struct {
	StepIndex int
	Workflow  string
	StepName  string
	Entries   []FilteredLogEntry
}

// FilteredLogEntry wraps a log entry with match information.
type FilteredLogEntry struct {
	Original      LogEntry
	OriginalIndex int
	Matches       []MatchPosition
}

// MatchPosition indicates where a search term was found in the content.
type MatchPosition struct {
	Start int
	End   int
}

// QuickFilters provides common filter configurations.
var QuickFilters = map[string]*FilterConfig{
	"all": {
		Level:         FilterAll,
		SearchTerm:    "",
		CaseSensitive: false,
		Regex:         false,
		StepIndex:     -1,
	},
	"errors": {
		Level:         FilterErrors,
		SearchTerm:    "",
		CaseSensitive: false,
		Regex:         false,
		StepIndex:     -1,
	},
	"warnings": {
		Level:         FilterWarnings,
		SearchTerm:    "",
		CaseSensitive: false,
		Regex:         false,
		StepIndex:     -1,
	},
}
