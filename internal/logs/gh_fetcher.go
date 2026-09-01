package logs

import (
	"bufio"
	"fmt"
	"strconv"
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

	for i := range jobs {
		job := &jobs[i]

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
		strconv.FormatInt(runID, 10),
		"--log",
		"--job", strconv.FormatInt(jobID, 10))
	if err != nil {
		return "", fmt.Errorf("gh command failed: %w (stderr: %s)", err, stderr)
	}

	return stdout, nil
}

// logFieldCount is how many tab-separated fields a `gh run view --log` line
// carries: the job name, the step name, and the timestamped message.
const logFieldCount = 3

// GitHub opens the first line of a downloaded log with a byte order mark.
const byteOrderMark = "\ufeff"

// parseJobLogsIntoSteps groups a job's log lines by the step each belongs to.
//
// `gh run view --log` prefixes every line with "job\tstep\ttimestamp message",
// which names the step directly. Splitting on ##[group] markers instead would
// find none, because those sit after the prefix rather than at the start of a
// line.
func (*GHFetcher) parseJobLogsIntoSteps(
	job *github.Job,
	rawLogs string,
	workflow string,
	runID int64,
	startIndex int,
) []*StepLogs {
	steps := make(map[string]github.Step, len(job.Steps))
	for _, step := range job.Steps {
		steps[step.Name] = step
	}

	order, byStep := groupLogLinesByStep(steps, rawLogs)
	if len(order) == 0 {
		return nil
	}

	stepLogs := make([]*StepLogs, 0, len(order))

	for i, name := range order {
		step, declared := steps[name]

		stepName := name
		if !declared {
			// gh could not resolve these lines to a step, so they carry the
			// job's own outcome rather than none.
			stepName = job.Name
			step = github.Step{Status: job.Status, Conclusion: job.Conclusion}
		}

		stepLogs = append(stepLogs, &StepLogs{
			StepIndex:  startIndex + i,
			Workflow:   workflow,
			RunID:      runID,
			JobName:    job.Name,
			StepName:   stepName,
			Status:     step.Status,
			Conclusion: step.Conclusion,
			Entries:    ParseLogOutput(strings.Join(byStep[name], "\n"), stepName),
			FetchedAt:  time.Now(),
		})
	}

	return stepLogs
}

// groupLogLinesByStep returns the step names in the order they first appear and
// the message lines belonging to each.
//
// A line opens a step only when its second field names one the job declares.
// Requiring that, rather than trusting any three tab-separated fields, is what
// stops a log message carrying tabs of its own (a Go test summary, a formatted
// table) from inventing a step. A line that opens nothing belongs to the step
// already open.
func groupLogLinesByStep(steps map[string]github.Step, rawLogs string) ([]string, map[string][]string) {
	var order []string

	byStep := make(map[string][]string)
	current := ""

	scanner := bufio.NewScanner(strings.NewReader(rawLogs))
	scanner.Buffer(make([]byte, 0, logScanBuffer), logScanBuffer)

	for scanner.Scan() {
		line := strings.TrimPrefix(scanner.Text(), byteOrderMark)

		name, message, ok := splitPrefix(line, steps)
		if !ok {
			if current != "" {
				byStep[current] = append(byStep[current], line)
			}

			continue
		}

		if _, seen := byStep[name]; !seen {
			order = append(order, name)
		}

		current = name
		byStep[name] = append(byStep[name], message)
	}

	return order, byStep
}

// splitPrefix reads one "job\tstep\ttimestamp message" line, returning the
// step it belongs to and the message. It reports false for a line that names no
// declared step.
func splitPrefix(line string, steps map[string]github.Step) (string, string, bool) {
	fields := strings.SplitN(line, "\t", logFieldCount)
	if len(fields) < logFieldCount {
		return "", "", false
	}

	name := fields[1]
	if _, declared := steps[name]; !declared && name != unresolvedStep {
		return "", "", false
	}

	return name, strings.TrimPrefix(fields[2], byteOrderMark), true
}

// unresolvedStep is what gh writes in the step field when it cannot map a
// line to a declared step, which it does for every line of some jobs. Treating
// it as a step of its own is what keeps those jobs from parsing to nothing;
// only a sentinel this exact is trusted, so a message carrying tabs still
// cannot invent a step.
const unresolvedStep = "UNKNOWN STEP"

// logScanBuffer bounds a single log line. A stack trace or a base64 payload on
// one line runs well past bufio's 64KB default, and a line over the limit would
// silently end the scan.
const logScanBuffer = 1024 * 1024

// FetchWorkflowLogs fetches all logs for a workflow run (all jobs).
func (f *GHFetcher) FetchWorkflowLogs(runID int64) (string, error) {
	// Use gh CLI to view all logs
	// Command: gh run view <run-id> --log
	stdout, stderr, err := f.executor.Execute("gh", "run", "view",
		strconv.FormatInt(runID, 10),
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
