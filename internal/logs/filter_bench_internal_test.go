package logs

import (
	"testing"
)

func BenchmarkFilter_Apply_10kEntries(b *testing.B) {
	runLogs := GenerateRunLogsWithEntries(10000)
	config := &FilterConfig{
		Level:      FilterErrors,
		SearchTerm: "",
		StepIndex:  -1,
	}
	filter, _ := NewFilter(config)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		filter.Apply(runLogs)
	}
}

func BenchmarkFilter_Apply_50kEntries(b *testing.B) {
	runLogs := GenerateRunLogsWithEntries(50000)
	config := &FilterConfig{
		Level:      FilterErrors,
		SearchTerm: "",
		StepIndex:  -1,
	}
	filter, _ := NewFilter(config)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		filter.Apply(runLogs)
	}
}

func BenchmarkFilter_SearchTerm_10kEntries(b *testing.B) {
	runLogs := GenerateRunLogsWithEntries(10000)
	config := &FilterConfig{
		Level:         FilterAll,
		SearchTerm:    "error",
		CaseSensitive: false,
		Regex:         false,
		StepIndex:     -1,
	}
	filter, _ := NewFilter(config)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		filter.Apply(runLogs)
	}
}

func BenchmarkFilter_RegexSearch_10kEntries(b *testing.B) {
	runLogs := GenerateRunLogsWithEntries(10000)
	config := &FilterConfig{
		Level:         FilterAll,
		SearchTerm:    `\b(error|failed|failure)\b`,
		Regex:         true,
		CaseSensitive: false,
		StepIndex:     -1,
	}
	filter, _ := NewFilter(config)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		filter.Apply(runLogs)
	}
}

func BenchmarkFilter_RegexSearch_50kEntries(b *testing.B) {
	runLogs := GenerateRunLogsWithEntries(50000)
	config := &FilterConfig{
		Level:         FilterAll,
		SearchTerm:    `\b(error|failed|failure)\b`,
		Regex:         true,
		CaseSensitive: false,
		StepIndex:     -1,
	}
	filter, _ := NewFilter(config)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		filter.Apply(runLogs)
	}
}

func BenchmarkFilter_CombinedFilters_10kEntries(b *testing.B) {
	runLogs := GenerateRunLogsWithEntries(10000)
	config := &FilterConfig{
		Level:         FilterErrors,
		SearchTerm:    "error",
		CaseSensitive: false,
		Regex:         false,
		StepIndex:     -1,
	}
	filter, _ := NewFilter(config)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		filter.Apply(runLogs)
	}
}

func BenchmarkFilter_FindMatches_SimpleString(b *testing.B) {
	config := &FilterConfig{
		SearchTerm:    "error",
		CaseSensitive: false,
		Regex:         false,
	}
	filter, _ := NewFilter(config)

	content := "This is an error message with error in it multiple error times"

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		filter.findMatches(content)
	}
}

func BenchmarkFilter_FindMatches_Regex(b *testing.B) {
	config := &FilterConfig{
		SearchTerm:    `\b(error|warning|failed)\b`,
		Regex:         true,
		CaseSensitive: false,
	}
	filter, _ := NewFilter(config)

	content := "This is an error message with warning in it and failed multiple error times"

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		filter.findMatches(content)
	}
}

func BenchmarkFilter_NewFilter_Regex(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		config := &FilterConfig{
			SearchTerm:    `\b(error|warning|failed)\b`,
			Regex:         true,
			CaseSensitive: false,
		}
		NewFilter(config)
	}
}

func BenchmarkFilter_StepIndexFilter_10kEntries(b *testing.B) {
	runLogs := GenerateRunLogsWithEntries(10000)
	config := &FilterConfig{
		Level:     FilterAll,
		StepIndex: 5, // Filter to single step
	}
	filter, _ := NewFilter(config)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		filter.Apply(runLogs)
	}
}

func BenchmarkFilteredResult_TotalEntries(b *testing.B) {
	// Create a large filtered result
	result := &FilteredResult{
		Steps: make([]*FilteredStepLogs, 100),
	}

	for i := range result.Steps {
		result.Steps[i] = &FilteredStepLogs{
			Entries: make([]FilteredLogEntry, 100),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		result.TotalEntries()
	}
}

func BenchmarkFilter_CaseSensitiveSearch_10kEntries(b *testing.B) {
	runLogs := GenerateRunLogsWithEntries(10000)
	config := &FilterConfig{
		Level:         FilterAll,
		SearchTerm:    "ERROR",
		CaseSensitive: true,
		Regex:         false,
		StepIndex:     -1,
	}
	filter, _ := NewFilter(config)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		filter.Apply(runLogs)
	}
}

func BenchmarkFilter_WarningsFilter_10kEntries(b *testing.B) {
	runLogs := GenerateRunLogsWithEntries(10000)
	config := &FilterConfig{
		Level:     FilterWarnings,
		StepIndex: -1,
	}
	filter, _ := NewFilter(config)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		filter.Apply(runLogs)
	}
}
