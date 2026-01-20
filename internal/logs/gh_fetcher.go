package logs

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// GHFetcher fetches real logs using gh CLI.
type GHFetcher struct {
	client   GitHubClient
	executor exec.CommandExecutor
}

// NewGHFetcher creates a fetcher that uses gh CLI for real log access.
func NewGHFetcher(client GitHubClient) *GHFetcher {
	return &GHFetcher{
		client:   client,
		executor: exec.NewRealExecutor(),
	}
}

// NewGHFetcherWithExecutor creates a fetcher with a custom executor (for testing).
func NewGHFetcherWithExecutor(client GitHubClient, executor exec.CommandExecutor) *GHFetcher {
	return &GHFetcher{
		client:   client,
		executor: executor,
	}
}

// FetchStepLogsReal fetches actual logs from GitHub using gh CLI.
func (f *GHFetcher) FetchStepLogsReal(runID int64, workflow string) ([]*StepLogs, error) {
	// First, get job metadata from API
	jobs, err := f.client.GetWorkflowRunJobs(runID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch jobs: %w", err)
	}

	var allStepLogs []*StepLogs
	stepIndex := 0

	for _, job := range jobs {
		// Fetch logs for this job using gh CLI
		jobLogs, err := f.fetchJobLogs(runID, job.ID)
		if err != nil {
			// Store error but continue with other jobs
			for _, step := range job.Steps {
				allStepLogs = append(allStepLogs, &StepLogs{
					StepIndex:  stepIndex,
					Workflow:   workflow,
					RunID:      runID,
					JobName:    job.Name,
					StepName:   step.Name,
					Status:     step.Status,
					Conclusion: step.Conclusion,
					Error:      err,
					FetchedAt:  time.Now(),
				})
				stepIndex++
			}
			continue
		}

		// Parse logs into steps
		stepLogs := f.parseJobLogsIntoSteps(job, jobLogs, workflow, runID, stepIndex)
		allStepLogs = append(allStepLogs, stepLogs...)
		stepIndex += len(stepLogs)
	}

	return allStepLogs, nil
}

// fetchJobLogs uses gh CLI to download logs for a specific job.
func (f *GHFetcher) fetchJobLogs(runID, jobID int64) (string, error) {
	// Use gh CLI to view logs
	// Command: gh run view <run-id> --log --job <job-id>
	stdout, stderr, err := f.executor.Execute("gh", "run", "view",
		fmt.Sprintf("%d", runID),
		"--log",
		"--job", fmt.Sprintf("%d", jobID))

	if err != nil {
		return "", fmt.Errorf("gh command failed: %w (stderr: %s)", err, stderr)
	}

	return stdout, nil
}

// parseJobLogsIntoSteps parses raw job logs into separate step logs.
func (f *GHFetcher) parseJobLogsIntoSteps(
	job github.Job,
	rawLogs string,
	workflow string,
	runID int64,
	startIndex int,
) []*StepLogs {
	// GitHub logs format:
	// ##[group]Run actions/checkout@v4
	// ... log lines ...
	// ##[endgroup]
	//
	// ##[group]Install dependencies
	// ... log lines ...
	// ##[endgroup]

	var stepLogs []*StepLogs
	scanner := bufio.NewScanner(strings.NewReader(rawLogs))

	currentStepIdx := -1
	var currentLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// Detect step boundaries
		if strings.HasPrefix(line, "##[group]") {
			// Save previous step if any
			if currentStepIdx >= 0 && currentStepIdx < len(job.Steps) {
				step := job.Steps[currentStepIdx]
				stepLogs = append(stepLogs, &StepLogs{
					StepIndex:  startIndex + currentStepIdx,
					Workflow:   workflow,
					RunID:      runID,
					JobName:    job.Name,
					StepName:   step.Name,
					Status:     step.Status,
					Conclusion: step.Conclusion,
					Entries:    ParseLogOutput(strings.Join(currentLines, "\n"), step.Name),
					FetchedAt:  time.Now(),
				})
			}

			// Start new step
			currentStepIdx++
			currentLines = make([]string, 0)
			currentLines = append(currentLines, line)
		} else if strings.HasPrefix(line, "##[endgroup]") {
			currentLines = append(currentLines, line)
		} else {
			// Regular log line
			currentLines = append(currentLines, line)
		}
	}

	// Save last step
	if currentStepIdx >= 0 && currentStepIdx < len(job.Steps) {
		step := job.Steps[currentStepIdx]
		stepLogs = append(stepLogs, &StepLogs{
			StepIndex:  startIndex + currentStepIdx,
			Workflow:   workflow,
			RunID:      runID,
			JobName:    job.Name,
			StepName:   step.Name,
			Status:     step.Status,
			Conclusion: step.Conclusion,
			Entries:    ParseLogOutput(strings.Join(currentLines, "\n"), step.Name),
			FetchedAt:  time.Now(),
		})
	}

	return stepLogs
}

// FetchWorkflowLogs fetches all logs for a workflow run (all jobs).
func (f *GHFetcher) FetchWorkflowLogs(runID int64) (string, error) {
	// Use gh CLI to view all logs
	// Command: gh run view <run-id> --log
	stdout, stderr, err := f.executor.Execute("gh", "run", "view",
		fmt.Sprintf("%d", runID),
		"--log")

	if err != nil {
		return "", fmt.Errorf("gh command failed: %w (stderr: %s)", err, stderr)
	}

	return stdout, nil
}

// CheckGHCLIAvailable checks if gh CLI is installed and authenticated.
func CheckGHCLIAvailable() error {
	return CheckGHCLIAvailableWithExecutor(exec.NewRealExecutor())
}

// CheckGHCLIAvailableWithExecutor checks if gh CLI is installed and authenticated using a custom executor.
func CheckGHCLIAvailableWithExecutor(executor exec.CommandExecutor) error {
	// Check if gh is installed
	_, _, err := executor.Execute("gh", "--version")
	if err != nil {
		return fmt.Errorf("gh CLI not found: %w (install from https://cli.github.com)", err)
	}

	// Check if authenticated
	_, _, err = executor.Execute("gh", "auth", "status")
	if err != nil {
		return fmt.Errorf("gh CLI not authenticated: %w (run 'gh auth login')", err)
	}

	return nil
}
