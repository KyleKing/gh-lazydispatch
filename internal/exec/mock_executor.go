package exec

import (
	"fmt"
	"strings"
)

// MockExecutor simulates command execution for testing.
type MockExecutor struct {
	// Commands maps command patterns to responses.
	// Key format: "command arg1 arg2"
	Commands map[string]*CommandResult

	// DefaultResult is returned when no specific command matches.
	DefaultResult *CommandResult

	// ExecutedCommands tracks all commands that were executed.
	ExecutedCommands []ExecutedCommand
}

// CommandResult represents the result of a command execution.
type CommandResult struct {
	Stdout string
	Stderr string
	Error  error
}

// ExecutedCommand tracks a command that was executed.
type ExecutedCommand struct {
	Name string
	Args []string
}

// NewMockExecutor creates a new mock executor.
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		Commands:         make(map[string]*CommandResult),
		ExecutedCommands: make([]ExecutedCommand, 0),
	}
}

// Execute simulates command execution by looking up the command in the Commands map.
func (m *MockExecutor) Execute(name string, args ...string) (string, string, error) {
	// Track the executed command
	m.ExecutedCommands = append(m.ExecutedCommands, ExecutedCommand{
		Name: name,
		Args: args,
	})

	// Build command key
	cmdKey := m.buildCommandKey(name, args)

	// Look for exact match
	if result, ok := m.Commands[cmdKey]; ok {
		return result.Stdout, result.Stderr, result.Error
	}

	// Look for pattern match (allows wildcards)
	for pattern, result := range m.Commands {
		if m.matchesPattern(cmdKey, pattern) {
			return result.Stdout, result.Stderr, result.Error
		}
	}

	// Use default if available
	if m.DefaultResult != nil {
		return m.DefaultResult.Stdout, m.DefaultResult.Stderr, m.DefaultResult.Error
	}

	// No match found
	return "", "", fmt.Errorf("mock executor: no result configured for command: %s", cmdKey)
}

// AddCommand registers a command response.
func (m *MockExecutor) AddCommand(name string, args []string, stdout, stderr string, err error) {
	cmdKey := m.buildCommandKey(name, args)
	m.Commands[cmdKey] = &CommandResult{
		Stdout: stdout,
		Stderr: stderr,
		Error:  err,
	}
}

// AddGHRunView is a convenience method for adding gh run view commands.
func (m *MockExecutor) AddGHRunView(runID int64, jobID int64, logOutput string) {
	args := []string{"run", "view", fmt.Sprintf("%d", runID), "--log"}
	if jobID > 0 {
		args = append(args, "--job", fmt.Sprintf("%d", jobID))
	}
	m.AddCommand("gh", args, logOutput, "", nil)
}

// AddGHRunViewError is a convenience method for adding failing gh run view commands.
func (m *MockExecutor) AddGHRunViewError(runID int64, jobID int64, stderr string, err error) {
	args := []string{"run", "view", fmt.Sprintf("%d", runID), "--log"}
	if jobID > 0 {
		args = append(args, "--job", fmt.Sprintf("%d", jobID))
	}
	m.AddCommand("gh", args, "", stderr, err)
}

// Reset clears all command history and configurations.
func (m *MockExecutor) Reset() {
	m.Commands = make(map[string]*CommandResult)
	m.ExecutedCommands = make([]ExecutedCommand, 0)
	m.DefaultResult = nil
}

// AddGHWorkflowRun mocks a successful gh workflow run command.
func (m *MockExecutor) AddGHWorkflowRun(workflow, branch string, inputs map[string]string) {
	args := []string{"workflow", "run", workflow}
	if branch != "" {
		args = append(args, "--ref", branch)
	}
	for k, v := range inputs {
		if v != "" {
			args = append(args, "-f", k+"="+v)
		}
	}
	m.AddCommand("gh", args, "", "", nil)
}

// AddGHWorkflowRunError mocks a failing gh workflow run command.
func (m *MockExecutor) AddGHWorkflowRunError(workflow, branch string, stderr string, err error) {
	args := []string{"workflow", "run", workflow}
	if branch != "" {
		args = append(args, "--ref", branch)
	}
	m.AddCommand("gh", args, "", stderr, err)
}

// AddGHAPIRun mocks a gh api call for workflow run data.
func (m *MockExecutor) AddGHAPIRun(owner, repo string, runID int64, status, conclusion string) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs/%d", owner, repo, runID)
	runJSON := fmt.Sprintf(`{"id":%d,"name":"CI","status":"%s","conclusion":"%s","html_url":"https://github.com/%s/%s/actions/runs/%d"}`,
		runID, status, conclusion, owner, repo, runID)
	m.AddCommand("gh", []string{"api", path}, runJSON, "", nil)
}

// AddGHAPIJobs mocks a gh api call for workflow run jobs.
func (m *MockExecutor) AddGHAPIJobs(owner, repo string, runID int64, jobs string) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs/%d/jobs", owner, repo, runID)
	m.AddCommand("gh", []string{"api", path}, jobs, "", nil)
}

// AddGHAPILatestRun mocks a gh api call for the latest workflow run.
func (m *MockExecutor) AddGHAPILatestRun(owner, repo, workflow string, runID int64, status string) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs?per_page=1", owner, repo)
	if workflow != "" {
		path += "&workflow=" + workflow
	}
	runsJSON := fmt.Sprintf(`{"total_count":1,"workflow_runs":[{"id":%d,"name":"CI","status":"%s"}]}`, runID, status)
	m.AddCommand("gh", []string{"api", path}, runsJSON, "", nil)
}

// AddGHVersion mocks the gh --version command.
func (m *MockExecutor) AddGHVersion(version string) {
	m.AddCommand("gh", []string{"--version"}, fmt.Sprintf("gh version %s (2024-01-01)", version), "", nil)
}

// AddGHAuthStatus mocks the gh auth status command.
func (m *MockExecutor) AddGHAuthStatus(authenticated bool, username string) {
	if authenticated {
		m.AddCommand("gh", []string{"auth", "status"}, fmt.Sprintf("✓ Logged in to github.com as %s", username), "", nil)
	} else {
		m.AddCommand("gh", []string{"auth", "status"}, "", "You are not logged in", fmt.Errorf("exit status 1"))
	}
}

// buildCommandKey creates a string key from command name and args.
func (m *MockExecutor) buildCommandKey(name string, args []string) string {
	parts := append([]string{name}, args...)
	return strings.Join(parts, " ")
}

// matchesPattern checks if a command matches a pattern (simple wildcard support).
func (m *MockExecutor) matchesPattern(cmd, pattern string) bool {
	// Simple wildcard matching: * matches any segment
	if !strings.Contains(pattern, "*") {
		return cmd == pattern
	}

	patternParts := strings.Split(pattern, " ")
	cmdParts := strings.Split(cmd, " ")

	if len(patternParts) != len(cmdParts) {
		return false
	}

	for i, pp := range patternParts {
		if pp == "*" {
			continue
		}
		if pp != cmdParts[i] {
			return false
		}
	}

	return true
}
