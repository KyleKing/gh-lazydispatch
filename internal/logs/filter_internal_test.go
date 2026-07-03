package logs

import (
	"testing"
)

func TestFilter_NewFilter(t *testing.T) {
	config := &FilterConfig{
		Level:         FilterAll,
		SearchTerm:    "",
		CaseSensitive: false,
		Regex:         false,
		StepIndex:     -1,
	}

	filter, err := NewFilter(config)
	if err != nil {
		t.Fatalf("NewFilter failed: %v", err)
	}

	if filter == nil {
		t.Fatal("expected non-nil filter")
	}

	if filter.config != config {
		t.Error("config mismatch")
	}
}

func TestFilter_NewFilterWithRegex(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		expectError   bool
		caseSensitive bool
	}{
		{"valid regex", `\d+`, false, false},
		{"invalid regex", `[`, true, false},
		{"case insensitive", `ERROR`, false, false},
		{"case sensitive", `ERROR`, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FilterConfig{
				Level:         FilterAll,
				SearchTerm:    tt.pattern,
				Regex:         true,
				CaseSensitive: tt.caseSensitive,
			}

			filter, err := NewFilter(config)
			if (err != nil) != tt.expectError {
				t.Errorf("error: got %v, expectError=%v", err, tt.expectError)
			}

			if !tt.expectError && filter == nil {
				t.Error("expected non-nil filter for valid regex")
			}

			if !tt.expectError && filter.regex == nil {
				t.Error("expected compiled regex")
			}
		})
	}
}

func TestFilter_ApplyByLevel(t *testing.T) {
	runLogs := NewRunLogs("test", "main")
	runLogs.AddStep(&StepLogs{
		StepIndex: 0,
		StepName:  "build",
		Entries: []LogEntry{
			{Content: "info line", Level: LogLevelInfo},
			{Content: "warning line", Level: LogLevelWarning},
			{Content: "error line", Level: LogLevelError},
			{Content: "debug line", Level: LogLevelDebug},
		},
	})

	tests := []struct {
		name          string
		level         FilterLevel
		expectedCount int
	}{
		{"all levels", FilterAll, 4},
		{"errors only", FilterErrors, 1},
		{"warnings and errors", FilterWarnings, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FilterConfig{
				Level:      tt.level,
				SearchTerm: "",
				StepIndex:  -1,
			}

			filter, err := NewFilter(config)
			if err != nil {
				t.Fatalf("NewFilter failed: %v", err)
			}

			result := filter.Apply(runLogs)
			if result.TotalEntries() != tt.expectedCount {
				t.Errorf("expected %d entries, got %d", tt.expectedCount, result.TotalEntries())
			}
		})
	}
}

func TestFilter_SearchTerm(t *testing.T) {
	runLogs := NewRunLogs("test", "main")
	runLogs.AddStep(&StepLogs{
		StepIndex: 0,
		StepName:  "build",
		Entries: []LogEntry{
			{Content: "Starting build process", Level: LogLevelInfo},
			{Content: "Build completed successfully", Level: LogLevelInfo},
			{Content: "Running tests", Level: LogLevelInfo},
			{Content: "Test failed", Level: LogLevelError},
		},
	})

	tests := []struct {
		name          string
		searchTerm    string
		caseSensitive bool
		expectedCount int
	}{
		{"case insensitive match", "build", false, 2},
		{"case sensitive match", "Build", true, 1},
		{"no match", "deploy", false, 0},
		{"partial match", "test", false, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FilterConfig{
				Level:         FilterAll,
				SearchTerm:    tt.searchTerm,
				CaseSensitive: tt.caseSensitive,
				Regex:         false,
				StepIndex:     -1,
			}

			filter, err := NewFilter(config)
			if err != nil {
				t.Fatalf("NewFilter failed: %v", err)
			}

			result := filter.Apply(runLogs)
			if result.TotalEntries() != tt.expectedCount {
				t.Errorf("expected %d entries, got %d", tt.expectedCount, result.TotalEntries())
			}
		})
	}
}

func TestFilter_RegexMatching(t *testing.T) {
	runLogs := NewRunLogs("test", "main")
	runLogs.AddStep(&StepLogs{
		StepIndex: 0,
		StepName:  "build",
		Entries: []LogEntry{
			{Content: "line 123", Level: LogLevelInfo},
			{Content: "error occurred", Level: LogLevelError},
			{Content: "ERROR: failed", Level: LogLevelError},
			{Content: "warning: check this", Level: LogLevelWarning},
		},
	})

	tests := []struct {
		name          string
		pattern       string
		caseSensitive bool
		expectedCount int
		expectError   bool
	}{
		{"digit pattern", `\d+`, false, 1, false},
		{"word boundary", `\berror\b`, false, 2, false},
		{"case insensitive", `ERROR`, false, 2, false},
		{"invalid pattern", `[`, false, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FilterConfig{
				Level:         FilterAll,
				SearchTerm:    tt.pattern,
				Regex:         true,
				CaseSensitive: tt.caseSensitive,
				StepIndex:     -1,
			}

			filter, err := NewFilter(config)
			if (err != nil) != tt.expectError {
				t.Errorf("error: got %v, expectError=%v", err, tt.expectError)
				return
			}

			if tt.expectError {
				return
			}

			result := filter.Apply(runLogs)
			if result.TotalEntries() != tt.expectedCount {
				t.Errorf("expected %d entries, got %d", tt.expectedCount, result.TotalEntries())
			}
		})
	}
}

func TestFilter_StepIndexFilter(t *testing.T) {
	runLogs := NewRunLogs("test", "main")
	runLogs.AddStep(&StepLogs{
		StepIndex: 0,
		StepName:  "checkout",
		Entries:   []LogEntry{{Content: "checkout logs", Level: LogLevelInfo}},
	})
	runLogs.AddStep(&StepLogs{
		StepIndex: 1,
		StepName:  "build",
		Entries:   []LogEntry{{Content: "build logs", Level: LogLevelInfo}},
	})
	runLogs.AddStep(&StepLogs{
		StepIndex: 2,
		StepName:  "test",
		Entries:   []LogEntry{{Content: "test logs", Level: LogLevelInfo}},
	})

	tests := []struct {
		name          string
		stepIndex     int
		expectedSteps int
	}{
		{"all steps", -1, 3},
		{"step 0", 0, 1},
		{"step 1", 1, 1},
		{"step 2", 2, 1},
		{"non-existent step", 99, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FilterConfig{
				Level:     FilterAll,
				StepIndex: tt.stepIndex,
			}

			filter, err := NewFilter(config)
			if err != nil {
				t.Fatalf("NewFilter failed: %v", err)
			}

			result := filter.Apply(runLogs)
			if len(result.Steps) != tt.expectedSteps {
				t.Errorf("expected %d steps, got %d", tt.expectedSteps, len(result.Steps))
			}
		})
	}
}

func TestFilter_FindMatches(t *testing.T) {
	tests := []struct {
		name          string
		searchTerm    string
		content       string
		regex         bool
		caseSensitive bool
		expectedCount int
	}{
		{"single match", "test", "this is a test", false, false, 1},
		{"multiple matches", "test", "test test test", false, false, 3},
		{"case insensitive", "TEST", "test Test TEST", false, false, 3},
		{"case sensitive", "TEST", "test Test TEST", false, true, 1},
		{"no match", "missing", "this is a test", false, false, 0},
		{"regex match", `\d+`, "line 123 and 456", true, false, 2},
		{"empty search term", "", "any content", false, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FilterConfig{
				SearchTerm:    tt.searchTerm,
				Regex:         tt.regex,
				CaseSensitive: tt.caseSensitive,
			}

			filter, err := NewFilter(config)
			if err != nil {
				t.Fatalf("NewFilter failed: %v", err)
			}

			matches := filter.findMatches(tt.content)
			if len(matches) != tt.expectedCount {
				t.Errorf("expected %d matches, got %d", tt.expectedCount, len(matches))
			}

			// Verify match positions are valid
			for _, match := range matches {
				if match.Start < 0 || match.End > len(tt.content) || match.Start >= match.End {
					t.Errorf("invalid match position: Start=%d, End=%d, content len=%d",
						match.Start, match.End, len(tt.content))
				}
			}
		})
	}
}

func TestFilter_EmptyResults(t *testing.T) {
	runLogs := NewRunLogs("test", "main")
	runLogs.AddStep(&StepLogs{
		StepIndex: 0,
		StepName:  "build",
		Entries: []LogEntry{
			{Content: "info line", Level: LogLevelInfo},
			{Content: "debug line", Level: LogLevelDebug},
		},
	})

	config := &FilterConfig{
		Level:      FilterErrors, // Only errors
		SearchTerm: "",
		StepIndex:  -1,
	}

	filter, err := NewFilter(config)
	if err != nil {
		t.Fatalf("NewFilter failed: %v", err)
	}

	result := filter.Apply(runLogs)

	if result.TotalEntries() != 0 {
		t.Errorf("expected 0 entries, got %d", result.TotalEntries())
	}

	if len(result.Steps) != 0 {
		t.Errorf("expected 0 steps (no matching entries), got %d", len(result.Steps))
	}
}

func TestFilteredResult_TotalEntries(t *testing.T) {
	result := &FilteredResult{
		Steps: []*FilteredStepLogs{
			{Entries: make([]FilteredLogEntry, 5)},
			{Entries: make([]FilteredLogEntry, 3)},
			{Entries: make([]FilteredLogEntry, 7)},
		},
	}

	total := result.TotalEntries()
	expected := 15

	if total != expected {
		t.Errorf("TotalEntries: got %d, want %d", total, expected)
	}
}

func TestQuickFilters(t *testing.T) {
	tests := []struct {
		name       string
		filterName string
		wantLevel  FilterLevel
	}{
		{"all filter", "all", FilterAll},
		{"errors filter", "errors", FilterErrors},
		{"warnings filter", "warnings", FilterWarnings},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, ok := QuickFilters[tt.filterName]
			if !ok {
				t.Fatalf("quick filter %q not found", tt.filterName)
			}

			if config.Level != tt.wantLevel {
				t.Errorf("Level: got %v, want %v", config.Level, tt.wantLevel)
			}

			if config.SearchTerm != "" {
				t.Error("expected empty SearchTerm in quick filter")
			}

			if config.StepIndex != -1 {
				t.Errorf("StepIndex: got %d, want -1", config.StepIndex)
			}
		})
	}
}

func TestFilter_MatchPosition(t *testing.T) {
	config := &FilterConfig{
		SearchTerm:    "error",
		CaseSensitive: false,
		Regex:         false,
	}

	filter, err := NewFilter(config)
	if err != nil {
		t.Fatalf("NewFilter failed: %v", err)
	}

	content := "An error occurred and another ERROR happened"
	matches := filter.findMatches(content)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	// First match should be "error"
	if matches[0].Start != 3 || matches[0].End != 8 {
		t.Errorf("first match: got Start=%d End=%d, want Start=3 End=8",
			matches[0].Start, matches[0].End)
	}

	// Second match should be "ERROR"
	if matches[1].Start != 30 || matches[1].End != 35 {
		t.Errorf("second match: got Start=%d End=%d, want Start=30 End=35",
			matches[1].Start, matches[1].End)
	}
}

func TestFilteredLogEntry_Fields(t *testing.T) {
	original := LogEntry{
		Content: "test log line",
		Level:   LogLevelInfo,
	}

	matches := []MatchPosition{
		{Start: 0, End: 4},
	}

	entry := FilteredLogEntry{
		Original:      original,
		OriginalIndex: 42,
		Matches:       matches,
	}

	if entry.Original.Content != "test log line" {
		t.Error("Original content mismatch")
	}

	if entry.OriginalIndex != 42 {
		t.Errorf("OriginalIndex: got %d, want 42", entry.OriginalIndex)
	}

	if len(entry.Matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(entry.Matches))
	}
}

func TestFilter_CombinedFilters(t *testing.T) {
	runLogs := NewRunLogs("test", "main")
	runLogs.AddStep(&StepLogs{
		StepIndex: 0,
		StepName:  "build",
		Entries: []LogEntry{
			{Content: "build started", Level: LogLevelInfo},
			{Content: "build error: failed", Level: LogLevelError},
			{Content: "warning: deprecated", Level: LogLevelWarning},
			{Content: "fatal error", Level: LogLevelError},
		},
	})

	// Filter: errors only + search for "error"
	config := &FilterConfig{
		Level:      FilterErrors,
		SearchTerm: "error",
		Regex:      false,
	}

	filter, err := NewFilter(config)
	if err != nil {
		t.Fatalf("NewFilter failed: %v", err)
	}

	result := filter.Apply(runLogs)

	// Should match 2 entries: both error-level entries that contain "error"
	if result.TotalEntries() != 2 {
		t.Errorf("expected 2 entries, got %d", result.TotalEntries())
	}
}
