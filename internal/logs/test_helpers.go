package logs

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

const (
	percentScale      = 100
	secondsPerMinute  = 60
	warningRateOffset = 0.1
)

// GenerateRunLogsWithEntries creates RunLogs with N total entries for benchmarking.
// Distributes entries across multiple steps with varying log levels.
func GenerateRunLogsWithEntries(totalEntries int) *RunLogs {
	runLogs := NewRunLogs("test", "main")

	entriesPerStep := 100
	numSteps := (totalEntries + entriesPerStep - 1) / entriesPerStep

	for i := range numSteps {
		stepEntries := make([]LogEntry, 0, entriesPerStep)
		remaining := totalEntries - (i * entriesPerStep)
		thisStepEntries := entriesPerStep

		if remaining < entriesPerStep {
			thisStepEntries = remaining
		}

		for j := range thisStepEntries {
			level := LogLevelInfo
			if j%10 == 0 {
				level = LogLevelError
			} else if j%5 == 0 {
				level = LogLevelWarning
			}

			stepEntries = append(stepEntries, LogEntry{
				Content:  fmt.Sprintf("Log line %d in step %d", j, i),
				Level:    level,
				StepName: fmt.Sprintf("step-%d", i),
			})
		}

		runLogs.AddStep(&StepLogs{
			StepIndex: i,
			StepName:  fmt.Sprintf("step-%d", i),
			Entries:   stepEntries,
		})
	}

	return runLogs
}

// GenerateLargeLogFixture creates a realistic log file with N lines.
// Uses GitHub Actions log format patterns for authenticity.
func GenerateLargeLogFixture(lines int) string {
	var sb strings.Builder

	templates := []string{
		"##[group]Run actions/checkout@v4",
		"Syncing repository: owner/repo",
		"##[endgroup]",
		"##[group]Build",
		"Installing dependencies...",
		"Running build process...",
		"##[endgroup]",
		"##[group]Test",
		"Running test suite...",
		"Test passed: %d",
		"##[endgroup]",
		"INFO: Processing file %d",
		"DEBUG: Cache hit for key %d",
		"Completed step %d of %d",
	}

	for i := range lines {
		template := templates[i%len(templates)]
		if strings.Contains(template, "%d") {
			sb.WriteString(fmt.Sprintf(template, i))
		} else {
			sb.WriteString(template)
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// GenerateLargeLogWithErrors creates a log with error patterns.
// The errorRate is a float between 0 and 1 indicating percentage of lines that should be errors.
func GenerateLargeLogWithErrors(lines int, errorRate float64) string {
	var sb strings.Builder

	for i := range lines {
		switch {
		case float64(i%percentScale) < errorRate*percentScale:
			sb.WriteString(fmt.Sprintf("##[error]Error on line %d: operation failed\n", i))
		case float64(i%percentScale) < (errorRate+warningRateOffset)*percentScale:
			sb.WriteString(fmt.Sprintf("##[warning]Warning on line %d: deprecated usage\n", i))
		default:
			sb.WriteString(fmt.Sprintf("INFO: Processing line %d\n", i))
		}
	}

	return sb.String()
}

// GenerateUnicodeLog creates logs with unicode characters.
// Tests proper handling of international characters and emoji.
//
//nolint:gosmopolitan // intentional non-ASCII test fixture verifying unicode log handling
func GenerateUnicodeLog() string {
	return `##[group]Build 🏗️
Running tests ✓
Warning: deprecated ⚠️
Error: failed ✗
Progress: ░░░░░░░░░░ 50%
Emoji: 🚀 🎉 💻 🔥 ✨
Japanese: テスト成功
Chinese: 测试通过
Russian: тест пройден
##[endgroup]`
}

// GenerateANSILog creates logs with ANSI color codes.
// Tests proper handling of terminal color escape sequences.
func GenerateANSILog() string {
	return "\x1b[32mSuccess: Build completed\x1b[0m\n" +
		"\x1b[31mError: Test failed\x1b[0m\n" +
		"\x1b[33mWarning: Deprecated API\x1b[0m\n" +
		"\x1b[1;34mBold Blue: Information\x1b[0m\n"
}

// GenerateMixedLog creates a log with various patterns for comprehensive testing.
// Includes errors, warnings, unicode, ANSI codes, and normal logs.
func GenerateMixedLog(lines int) string {
	var sb strings.Builder

	patterns := []string{
		"##[group]Step Group",
		"##[endgroup]",
		"INFO: Normal log line",
		"##[error]Error: Something failed",
		"##[warning]Warning: Deprecated usage",
		"Test passed ✓",
		"Test failed ✗",
	}

	for i := range lines {
		pattern := patterns[i%len(patterns)]
		sb.WriteString(fmt.Sprintf("%s %d\n", pattern, i))
	}

	return sb.String()
}

// GenerateLogWithTimestamps creates log lines with timestamp prefixes.
// Tests timestamp parsing and display.
func GenerateLogWithTimestamps(lines int) string {
	var sb strings.Builder

	for i := range lines {
		timestamp := fmt.Sprintf(
			"2024-01-01T12:%02d:%02d.000Z", i/secondsPerMinute%secondsPerMinute, i%secondsPerMinute,
		)
		sb.WriteString(fmt.Sprintf("%s INFO: Log line %d\n", timestamp, i))
	}

	return sb.String()
}

// GenerateMultiStepLog creates a log output with multiple GitHub Actions steps.
// Simulates a real workflow run with step grouping.
func GenerateMultiStepLog(numSteps, linesPerStep int) string {
	var sb strings.Builder

	for i := range numSteps {
		sb.WriteString(fmt.Sprintf("##[group]Run step-%d\n", i))

		for j := range linesPerStep {
			switch {
			case j%20 == 0:
				sb.WriteString(fmt.Sprintf("##[error]Error in step %d line %d\n", i, j))
			case j%10 == 0:
				sb.WriteString(fmt.Sprintf("##[warning]Warning in step %d line %d\n", i, j))
			default:
				sb.WriteString(fmt.Sprintf("INFO: Step %d line %d\n", i, j))
			}
		}

		sb.WriteString("##[endgroup]\n")
	}

	return sb.String()
}

// LoadFixture loads a test fixture file from testdata.
// Helper function for tests and benchmarks.
func LoadFixture(tb testing.TB, filename string) string {
	tb.Helper()

	// Try multiple paths for flexibility
	paths := []string{
		"../../testdata/logs/" + filename,
		"testdata/logs/" + filename,
		"../testdata/logs/" + filename,
	}

	for _, path := range paths {
		data, err := os.ReadFile(path) //nolint:gosec // test-only, fixed testdata directory candidates
		if err == nil {
			return string(data)
		}
	}

	tb.Fatalf("failed to load fixture %s from any path", filename)

	return ""
}
