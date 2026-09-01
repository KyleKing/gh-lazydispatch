package logs

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// GitHubClient interface for fetching workflow data.
type GitHubClient interface {
	GetWorkflowRun(runID int64) (*github.WorkflowRun, error)
	GetWorkflowRunJobs(runID int64) ([]github.Job, error)
}

// Fetcher fetches and parses workflow logs.
type Fetcher struct {
	client GitHubClient
}

// NewFetcher creates a new log fetcher.
func NewFetcher(client GitHubClient) *Fetcher {
	return &Fetcher{client: client}
}

// FetchStepLogs fetches logs for a specific workflow run.
// Returns a StepLogs for each job step in the workflow.
func (f *Fetcher) FetchStepLogs(runID int64, workflow string) ([]*StepLogs, error) {
	jobs, err := f.client.GetWorkflowRunJobs(runID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch jobs: %w", err)
	}

	var allStepLogs []*StepLogs

	stepIndex := 0

	for i := range jobs {
		job := &jobs[i]

		for _, step := range job.Steps {
			stepLogs := &StepLogs{
				StepIndex:  stepIndex,
				Workflow:   workflow,
				RunID:      runID,
				JobName:    job.Name,
				StepName:   step.Name,
				Status:     step.Status,
				Conclusion: step.Conclusion,
				FetchedAt:  time.Now(),
			}

			// For now, we create synthetic log entries based on step status
			// Real implementation would fetch actual logs via gh CLI or API
			stepLogs.Entries = f.generateSyntheticLogs(job.Name, step)

			allStepLogs = append(allStepLogs, stepLogs)
			stepIndex++
		}
	}

	return allStepLogs, nil
}

// generateSyntheticLogs creates placeholder logs based on step metadata.
// Real implementation would parse actual log output from GitHub.
func (*Fetcher) generateSyntheticLogs(jobName string, step github.Step) []LogEntry {
	entries := []LogEntry{
		{
			Timestamp: time.Now(),
			Content:   "Starting step: " + step.Name,
			Level:     LogLevelInfo,
			StepName:  step.Name,
		},
	}

	// Add conclusion-based entries
	switch step.Conclusion {
	case github.ConclusionSuccess:
		entries = append(entries, LogEntry{
			Timestamp: time.Now(),
			Content:   fmt.Sprintf("Step completed successfully (job: %s)", jobName),
			Level:     LogLevelInfo,
			StepName:  step.Name,
		})
	case github.ConclusionFailure:
		entries = append(entries, LogEntry{
			Timestamp: time.Now(),
			Content:   "Error: Step failed - check workflow logs for details",
			Level:     LogLevelError,
			StepName:  step.Name,
		})
	case github.ConclusionSkipped:
		entries = append(entries, LogEntry{
			Timestamp: time.Now(),
			Content:   "Step was skipped",
			Level:     LogLevelInfo,
			StepName:  step.Name,
		})
	}

	return entries
}

// ParseLogOutput parses raw log text into LogEntry structs.
// Detects log levels based on common patterns.
func ParseLogOutput(rawLogs, stepName string) []LogEntry {
	var entries []LogEntry

	scanner := bufio.NewScanner(strings.NewReader(rawLogs))

	errorPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\berror\b`),
		regexp.MustCompile(`(?i)\bfailed\b`),
		regexp.MustCompile(`(?i)\bfailure\b`),
		regexp.MustCompile(`(?i)✗`),
	}

	warningPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bwarning\b`),
		regexp.MustCompile(`(?i)\bwarn\b`),
		regexp.MustCompile(`(?i)⚠`),
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		stamp, content := splitTimestamp(line)
		content = stripEscapes(content)
		level := detectLogLevel(content, errorPatterns, warningPatterns)

		entries = append(entries, LogEntry{
			Timestamp: stamp,
			Content:   content,
			Level:     level,
			StepName:  stepName,
		})
	}

	return entries
}

// caretEscape matches an SGR sequence in the caret notation GitHub stores its
// logs in: a literal "^[" rather than an ESC byte, so ansi.Strip cannot see it.
var caretEscape = regexp.MustCompile(`\^\[\[[0-9;]*[A-Za-z]`) //nolint:gochecknoglobals // compiled once

// stripEscapes removes the color GitHub writes into its own log lines. It is
// meaningless once the viewer styles the line or a markdown export fences it,
// and it widens every line it appears on. Both spellings occur: a real escape
// in streamed output, caret notation in a downloaded log.
func stripEscapes(content string) string {
	return caretEscape.ReplaceAllString(ansi.Strip(content), "")
}

// splitTimestamp lifts the RFC3339 stamp GitHub puts at the head of every log
// line into the entry's own field, so the viewer spends its width on the
// message and a filter never matches a digit in a timestamp. A line without one
// is returned whole, stamped with when it was read.
func splitTimestamp(line string) (time.Time, string) {
	stamp, rest, found := strings.Cut(line, " ")
	if !found {
		return time.Now(), line
	}

	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return time.Now(), line
	}

	return parsed, rest
}

// detectLogLevel determines the log level based on content.
func detectLogLevel(line string, errorPatterns, warningPatterns []*regexp.Regexp) LogLevel {
	for _, pattern := range errorPatterns {
		if pattern.MatchString(line) {
			return LogLevelError
		}
	}

	for _, pattern := range warningPatterns {
		if pattern.MatchString(line) {
			return LogLevelWarning
		}
	}

	// Check for debug indicators
	if strings.Contains(strings.ToLower(line), "debug") {
		return LogLevelDebug
	}

	return LogLevelInfo
}

// FetchRunSummary creates a summary of failed steps without full logs.
func (f *Fetcher) FetchRunSummary(runID int64) (string, error) {
	jobs, err := f.client.GetWorkflowRunJobs(runID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch jobs: %w", err)
	}

	var summary strings.Builder

	fmt.Fprintf(&summary, "Workflow Run #%d Summary\n\n", runID)

	hasFailures := false

	for i := range jobs {
		job := &jobs[i]

		if job.Conclusion != github.ConclusionSuccess {
			hasFailures = true

			fmt.Fprintf(&summary, "Job: %s (%s)\n", job.Name, job.Conclusion)

			for _, step := range job.Steps {
				if step.Conclusion != github.ConclusionSuccess && step.Conclusion != "" {
					fmt.Fprintf(&summary, "  ✗ %s: %s\n", step.Name, step.Conclusion)
				}
			}

			summary.WriteString("\n")
		}
	}

	if !hasFailures {
		summary.WriteString("All jobs completed successfully\n")
	}

	summary.WriteString("\nTo view full logs:\n")
	fmt.Fprintf(&summary, "  gh run view %d --log\n", runID)

	return summary.String(), nil
}
