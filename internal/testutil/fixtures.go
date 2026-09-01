// Package testutil provides testing utilities, fixtures, and mocks for unit tests.
package testutil

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const endGroupMarker = "##[endgroup]"

const (
	percentScale      = 100
	secondsPerMinute  = 60
	warningRateOffset = 0.1
)

// GenerateLargeLogFixture creates a realistic log file with N lines.
// Uses GitHub Actions log format patterns for authenticity.
func GenerateLargeLogFixture(lines int) string {
	var sb strings.Builder

	templates := []string{
		"##[group]Run actions/checkout@v4",
		"Syncing repository: owner/repo",
		endGroupMarker,
		"##[group]Build",
		"Installing dependencies...",
		"Running build process...",
		endGroupMarker,
		"##[group]Test",
		"Running test suite...",
		"Test passed: %d",
		endGroupMarker,
		"INFO: Processing file %d",
		"DEBUG: Cache hit for key %d",
		"Completed step %d of %d",
	}

	for i := range lines {
		template := templates[i%len(templates)]
		if strings.Contains(template, "%d") {
			fmt.Fprintf(&sb, template, i)
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
			fmt.Fprintf(&sb, "##[error]Error on line %d: operation failed\n", i)
		case float64(i%percentScale) < (errorRate+warningRateOffset)*percentScale:
			fmt.Fprintf(&sb, "##[warning]Warning on line %d: deprecated usage\n", i)
		default:
			fmt.Fprintf(&sb, "INFO: Processing line %d\n", i)
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
Arabic: اختبار ناجح
Korean: 테스트 성공
Greek: δοκιμή επιτυχής
Currency: € £ ¥ ₹
Math: ∑ ∏ √ ∞
Arrows: → ← ↑ ↓
##[endgroup]`
}

// GenerateANSILog creates logs with ANSI color codes.
// Tests proper handling of terminal color escape sequences.
// Includes ##[group] markers for proper parsing.
func GenerateANSILog() string {
	return `##[group]Test
2024-01-01T00:00:01Z ` + "\x1b[32mSuccess: Build completed\x1b[0m" + `
2024-01-01T00:00:02Z ` + "\x1b[31mError: Test failed\x1b[0m" + `
2024-01-01T00:00:03Z ` + "\x1b[33mWarning: Deprecated API\x1b[0m" + `
2024-01-01T00:00:04Z ` + "\x1b[1;34mBold Blue: Information\x1b[0m" + `
2024-01-01T00:00:05Z ` + "\x1b[36mCyan: Debug message\x1b[0m" + `
2024-01-01T00:00:06Z ` + "\x1b[35mMagenta: Trace\x1b[0m" + `
2024-01-01T00:00:07Z ` + "\x1b[1mBold: Important\x1b[0m" + `
2024-01-01T00:00:08Z ` + "\x1b[4mUnderline: Emphasized\x1b[0m" + `
2024-01-01T00:00:09Z ` + "\x1b[7mReverse: Highlighted\x1b[0m" + `
2024-01-01T00:00:10Z ` + "\x1b[0mReset: Normal text" + `
##[endgroup]`
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

// GenerateLogWithPatterns creates a log with specific searchable patterns.
// Useful for testing search and filter functionality.
func GenerateLogWithPatterns(lines int, patterns []string) string {
	var sb strings.Builder

	for i := range lines {
		pattern := patterns[i%len(patterns)]
		fmt.Fprintf(&sb, "Line %d: %s\n", i, pattern)
	}

	return sb.String()
}

// GenerateMultiStepLog creates a log output with multiple GitHub Actions steps.
// Simulates a real workflow run with step grouping.
func GenerateMultiStepLog(numSteps, linesPerStep int) string {
	var sb strings.Builder

	for i := range numSteps {
		fmt.Fprintf(&sb, "##[group]Run step-%d\n", i)

		for j := range linesPerStep {
			switch {
			case j%20 == 0:
				fmt.Fprintf(&sb, "##[error]Error in step %d line %d\n", i, j)
			case j%10 == 0:
				fmt.Fprintf(&sb, "##[warning]Warning in step %d line %d\n", i, j)
			default:
				fmt.Fprintf(&sb, "INFO: Step %d line %d\n", i, j)
			}
		}

		sb.WriteString("##[endgroup]\n")
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
		fmt.Fprintf(&sb, "%s INFO: Log line %d\n", timestamp, i)
	}

	return sb.String()
}

// GenerateMixedLog creates a log with various patterns for comprehensive testing.
// Includes errors, warnings, unicode, ANSI codes, and normal logs.
func GenerateMixedLog(lines int) string {
	var sb strings.Builder

	patterns := []string{
		"##[group]Step Group",
		endGroupMarker,
		"INFO: Normal log line",
		"##[error]Error: Something failed",
		"##[warning]Warning: Deprecated usage",
		"DEBUG: Cache hit 🎯",
		"\x1b[32mSuccess\x1b[0m",
		"\x1b[31mFailure\x1b[0m",
		"Processing files... ░░░░░░░░░░",
		"Test passed ✓",
		"Test failed ✗",
	}

	for i := range lines {
		pattern := patterns[i%len(patterns)]
		fmt.Fprintf(&sb, "%s %d\n", pattern, i)
	}

	return sb.String()
}

// logLineInterval spaces a fixture's timestamps so no two lines share one.
const logLineInterval = 137 * time.Millisecond

// AsGHRunViewLog prefixes each line of raw with the job name, step name, and a
// timestamp, the shape `gh run view --log` emits. A fixture without that prefix
// exercises no step-splitting at all, which is how a parser that never worked
// against real output stayed green.
func AsGHRunViewLog(job, step, raw string) string {
	var sb strings.Builder

	stamp := time.Date(2026, time.September, 1, 3, 17, 43, 0, time.UTC)

	for _, line := range strings.Split(strings.TrimSuffix(raw, "\n"), "\n") {
		fmt.Fprintf(&sb, "%s\t%s\t%s %s\n", job, step, stamp.Format(time.RFC3339Nano), line)
		stamp = stamp.Add(logLineInterval)
	}

	return sb.String()
}
