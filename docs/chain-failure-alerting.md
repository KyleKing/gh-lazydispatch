# Chain Failure Alerting and Log Access

## Current State

**Error Display** (internal/ui/modal/chain_status.go:159-163):
```go
if m.state.Error != nil {
    s.WriteString("\n")
    s.WriteString(ui.SelectedStyle.Render(fmt.Sprintf("Error: %s", m.state.Error.Error())))
    s.WriteString("\n")
}
```

**Limitations:**
- Only shows error string without context
- No link to workflow run logs
- No way to view detailed failure information
- No actionable guidance for resolution

## Proposed Enhancements

### 1. Enhanced Error Types

Create rich error types that capture workflow context for better alerting.

**File:** `internal/errors/chain_errors.go`

```go
package errors

import (
	"fmt"
	"strings"
)

// StepExecutionError represents a failure during chain step execution.
type StepExecutionError struct {
	StepIndex    int
	Workflow     string
	RunID        int64
	RunURL       string
	Conclusion   string
	ErrorMessage string
	Suggestion   string
}

func (e *StepExecutionError) Error() string {
	return fmt.Sprintf("step %d (%s) failed: %s", e.StepIndex+1, e.Workflow, e.ErrorMessage)
}

// DetailedMessage returns a formatted message with context and suggestions.
func (e *StepExecutionError) DetailedMessage() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Step %d failed: %s\n", e.StepIndex+1, e.Workflow))
	sb.WriteString(fmt.Sprintf("Status: %s\n", e.Conclusion))
	if e.ErrorMessage != "" {
		sb.WriteString(fmt.Sprintf("Error: %s\n", e.ErrorMessage))
	}
	if e.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("Suggestion: %s\n", e.Suggestion))
	}
	return sb.String()
}

// StepDispatchError represents a failure to dispatch a workflow.
type StepDispatchError struct {
	StepIndex    int
	Workflow     string
	ErrorMessage string
	Suggestion   string
}

func (e *StepDispatchError) Error() string {
	return fmt.Sprintf("failed to dispatch step %d (%s): %s", e.StepIndex+1, e.Workflow, e.ErrorMessage)
}

func (e *StepDispatchError) DetailedMessage() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Failed to dispatch: %s\n", e.Workflow))
	sb.WriteString(fmt.Sprintf("Error: %s\n", e.ErrorMessage))
	if e.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("Suggestion: %s", e.Suggestion))
	}
	return sb.String()
}

// InterpolationError represents a variable interpolation failure.
type InterpolationError struct {
	StepIndex  int
	Template   string
	Reason     string
	Suggestion string
}

func (e *InterpolationError) Error() string {
	return fmt.Sprintf("interpolation failed in step %d: %s", e.StepIndex+1, e.Reason)
}

func (e *InterpolationError) DetailedMessage() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Variable interpolation failed in step %d\n", e.StepIndex+1))
	sb.WriteString(fmt.Sprintf("Template: %s\n", e.Template))
	sb.WriteString(fmt.Sprintf("Reason: %s\n", e.Reason))
	if e.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("Suggestion: %s", e.Suggestion))
	}
	return sb.String()
}
```

### 2. Enhanced ChainExecutor Error Creation

Modify `internal/chain/executor.go` to create rich errors with context.

**In runStep()** (around line 231):
```go
func (e *ChainExecutor) runStep(idx int, step config.ChainStep) (*StepResult, error) {
	// ... existing interpolation code ...

	inputs, err := InterpolateInputs(step.Inputs, ctx)
	if err != nil {
		return nil, &errors.InterpolationError{
			StepIndex:  idx,
			Template:   fmt.Sprintf("%v", step.Inputs),
			Reason:     err.Error(),
			Suggestion: "Check that referenced variables exist and previous steps completed successfully",
		}
	}

	// ... existing dispatch code ...

	runID, err := runner.ExecuteAndGetRunID(cfg, e.client)
	if err != nil {
		return nil, &errors.StepDispatchError{
			StepIndex:    idx,
			Workflow:     step.Workflow,
			ErrorMessage: err.Error(),
			Suggestion:   "Verify workflow file exists and inputs are valid",
		}
	}

	// ... existing wait code ...

	conclusion, err := e.waitForRun(runID, step.WaitFor)
	if err != nil {
		return nil, err
	}

	status := StepCompleted
	if conclusion != github.ConclusionSuccess && step.WaitFor == config.WaitSuccess {
		status = StepFailed

		// Fetch the run to get URL and details
		run, err := e.client.GetWorkflowRun(runID)
		if err == nil {
			return &StepResult{
				Workflow:   step.Workflow,
				Inputs:     inputs,
				RunID:      runID,
				Status:     status,
				Conclusion: conclusion,
			}, &errors.StepExecutionError{
				StepIndex:    idx,
				Workflow:     step.Workflow,
				RunID:        runID,
				RunURL:       run.HTMLURL,
				Conclusion:   conclusion,
				ErrorMessage: fmt.Sprintf("Workflow concluded with: %s", conclusion),
				Suggestion:   "Check workflow logs for details",
			}
		}
	}

	return &StepResult{
		Workflow:   step.Workflow,
		Inputs:     inputs,
		RunID:      runID,
		Status:     status,
		Conclusion: conclusion,
	}, nil
}
```

### 3. Enhanced ChainStatusModal Display

Modify `internal/ui/modal/chain_status.go` to show rich error information.

**Add new methods:**
```go
// GetFailedStepRunURL returns the URL of the failed workflow run if available.
func (m *ChainStatusModal) GetFailedStepRunURL() string {
	if m.state.Error == nil {
		return ""
	}

	switch err := m.state.Error.(type) {
	case *errors.StepExecutionError:
		return err.RunURL
	default:
		return ""
	}
}

// GetDetailedError returns detailed error information if available.
func (m *ChainStatusModal) GetDetailedError() string {
	if m.state.Error == nil {
		return ""
	}

	type detailedError interface {
		DetailedMessage() string
	}

	if err, ok := m.state.Error.(detailedError); ok {
		return err.DetailedMessage()
	}

	return m.state.Error.Error()
}
```

**Enhanced View()** (replace lines 159-163):
```go
if m.state.Error != nil {
	s.WriteString("\n")
	s.WriteString(ui.ErrorTitleStyle.Render("Error Details:"))
	s.WriteString("\n")

	// Show detailed error message
	detailedMsg := m.GetDetailedError()
	for _, line := range strings.Split(detailedMsg, "\n") {
		s.WriteString(ui.ErrorStyle.Render("  " + line))
		s.WriteString("\n")
	}

	// Show workflow run URL if available
	if runURL := m.GetFailedStepRunURL(); runURL != "" {
		s.WriteString("\n")
		s.WriteString(ui.SubtitleStyle.Render("Logs: "))
		s.WriteString(ui.URLStyle.Render(runURL))
		s.WriteString("\n")
		s.WriteString(ui.HelpStyle.Render("  [l] view logs  [o] open in browser"))
		s.WriteString("\n")
	}
}
```

**Add new styles to** `internal/ui/styles.go`:
```go
var (
	// ... existing styles ...

	ErrorTitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("203")).
		Bold(true)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("203"))

	URLStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("111")).
		Underline(true)
)
```

### 4. Log Viewing Capabilities

Add commands to view logs directly from the error modal.

**New keymap in ChainStatusModal:**
```go
type chainStatusKeyMap struct {
	Close    key.Binding
	Stop     key.Binding
	Copy     key.Binding
	ViewLogs key.Binding  // NEW
	OpenURL  key.Binding  // NEW
}

func defaultChainStatusKeyMap() chainStatusKeyMap {
	return chainStatusKeyMap{
		Close:    key.NewBinding(key.WithKeys("esc", "q")),
		Stop:     key.NewBinding(key.WithKeys("ctrl+c")),
		Copy:     key.NewBinding(key.WithKeys("c")),
		ViewLogs: key.NewBinding(key.WithKeys("l")),
		OpenURL:  key.NewBinding(key.WithKeys("o")),
	}
}
```

**Enhanced Update() method:**
```go
func (m *ChainStatusModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		// ... existing cases ...

		case key.Matches(msg, m.keys.ViewLogs):
			if runURL := m.GetFailedStepRunURL(); runURL != "" {
				return m, m.fetchAndDisplayLogs()
			}

		case key.Matches(msg, m.keys.OpenURL):
			if runURL := m.GetFailedStepRunURL(); runURL != "" {
				return m, m.openURLInBrowser(runURL)
			}
		}
	}
	return m, nil
}

func (m *ChainStatusModal) fetchAndDisplayLogs() tea.Cmd {
	return func() tea.Msg {
		// Extract run ID from error
		var runID int64
		if err, ok := m.state.Error.(*errors.StepExecutionError); ok {
			runID = err.RunID
		} else {
			return nil
		}

		return FetchLogsMsg{RunID: runID}
	}
}

func (m *ChainStatusModal) openURLInBrowser(url string) tea.Cmd {
	return tea.Exec(
		exec.Command("open", url),
		func(err error) tea.Msg {
			if err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to open browser: %w", err)}
			}
			return nil
		},
	)
}
```

### 5. Local Log Fetching

Add functionality to fetch and display logs locally.

**File:** `internal/github/logs.go`

```go
package github

import (
	"fmt"
	"io"
)

// GetWorkflowRunLogs fetches the logs for a workflow run.
func (c *Client) GetWorkflowRunLogs(runID int64) (string, error) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs/%d/logs", c.owner, c.repo, runID)

	resp, err := c.rest.Request("GET", path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	defer resp.Body.Close()

	// GitHub returns a redirect to a zip file containing logs
	// For simplicity, we'll use gh CLI to download and extract
	return "", fmt.Errorf("log fetching not yet implemented - use gh run view %d --log", runID)
}

// GetWorkflowRunLogsSummary fetches a summary of failed jobs and steps.
func (c *Client) GetWorkflowRunLogsSummary(runID int64) (string, error) {
	jobs, err := c.GetWorkflowRunJobs(runID)
	if err != nil {
		return "", err
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Workflow Run #%d Summary\n\n", runID))

	for _, job := range jobs {
		if job.Conclusion != ConclusionSuccess {
			summary.WriteString(fmt.Sprintf("Job: %s (%s)\n", job.Name, job.Conclusion))

			for _, step := range job.Steps {
				if step.Conclusion != ConclusionSuccess && step.Conclusion != "" {
					summary.WriteString(fmt.Sprintf("  ✗ %s: %s\n", step.Name, step.Conclusion))
				}
			}
			summary.WriteString("\n")
		}
	}

	return summary.String(), nil
}
```

**Add new message type to** `internal/app/app.go`:
```go
// FetchLogsMsg requests fetching logs for a run.
type FetchLogsMsg struct {
	RunID int64
}

// LogsFetchedMsg contains fetched log summary.
type LogsFetchedMsg struct {
	RunID   int64
	Summary string
	Error   error
}
```

**Add handler in** `internal/app/handlers.go`:
```go
case FetchLogsMsg:
	return m, m.fetchLogsSummary(msg.RunID)

case LogsFetchedMsg:
	if msg.Error != nil {
		m.error = msg.Error
		return m, nil
	}
	// Display logs in a new modal or pane
	return m.showLogsModal(msg.RunID, msg.Summary), nil
```

**Add command:**
```go
func (m Model) fetchLogsSummary(runID int64) tea.Cmd {
	return func() tea.Msg {
		if m.ghClient == nil {
			return LogsFetchedMsg{
				RunID: runID,
				Error: fmt.Errorf("GitHub client not available"),
			}
		}

		summary, err := m.ghClient.GetWorkflowRunLogsSummary(runID)
		return LogsFetchedMsg{
			RunID:   runID,
			Summary: summary,
			Error:   err,
		}
	}
}
```

### 6. Log Viewer Modal

Create a new modal for displaying log summaries.

**File:** `internal/ui/modal/logs_viewer.go`

```go
package modal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// LogsViewerModal displays workflow run logs.
type LogsViewerModal struct {
	runID    int64
	summary  string
	viewport viewport.Model
	done     bool
	keys     logsViewerKeyMap
}

type logsViewerKeyMap struct {
	Close   key.Binding
	OpenWeb key.Binding
}

func defaultLogsViewerKeyMap() logsViewerKeyMap {
	return logsViewerKeyMap{
		Close:   key.NewBinding(key.WithKeys("esc", "q")),
		OpenWeb: key.NewBinding(key.WithKeys("o")),
	}
}

// NewLogsViewerModal creates a new logs viewer modal.
func NewLogsViewerModal(runID int64, summary string) *LogsViewerModal {
	vp := viewport.New(80, 20)
	vp.SetContent(summary)

	return &LogsViewerModal{
		runID:    runID,
		summary:  summary,
		viewport: vp,
		keys:     defaultLogsViewerKeyMap(),
	}
}

// Update handles input for the logs viewer modal.
func (m *LogsViewerModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Close):
			m.done = true
			return m, nil
		case key.Matches(msg, m.keys.OpenWeb):
			// Construct GitHub URL
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the logs viewer modal.
func (m *LogsViewerModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render(fmt.Sprintf("Workflow Run #%d - Failed Steps", m.runID)))
	s.WriteString("\n\n")

	s.WriteString(m.viewport.View())
	s.WriteString("\n\n")

	s.WriteString(ui.HelpStyle.Render("[↑↓] scroll  [o] open in browser  [esc/q] close"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *LogsViewerModal) IsDone() bool {
	return m.done
}

// Result returns nil.
func (m *LogsViewerModal) Result() any {
	return nil
}
```

## Implementation Plan

### Phase 1: Enhanced Error Types
1. Create `internal/errors/chain_errors.go` with rich error types
2. Update `ChainExecutor.runStep()` to create structured errors
3. Ensure errors include RunID and URL when available

### Phase 2: Improved Error Display
1. Add new styles to `internal/ui/styles.go`
2. Enhance `ChainStatusModal.View()` to show detailed errors
3. Display workflow run URLs prominently
4. Add visual hierarchy for error information

### Phase 3: Log Access
1. Add keybindings for log viewing and browser opening
2. Implement `GetWorkflowRunLogsSummary()` in GitHub client
3. Create `LogsViewerModal` for displaying summaries
4. Wire up message handling in app

### Phase 4: CLI Integration
1. Add suggestion to use `gh run view <run-id> --log` for full logs
2. Consider adding direct command execution from modal
3. Add ability to copy `gh run view` command to clipboard

## User Experience Flow

### Error Scenario: CI Gate Fails

1. **User triggers chain:**
   ```
   Chain: release-bump
   Status: running

   Steps:
   > * ci-gate.yml (running)
     o version-bump.yml (pending)

   [esc/q] close (continues)  [C-c] stop  [c] copy script
   ```

2. **CI gate fails:**
   ```
   Chain: release-bump
   Status: failed

   Steps:
     x ci-gate.yml (failed)
     - version-bump.yml (skipped)

   Error Details:
     Step 1 failed: ci-gate.yml
     Status: failure
     Error: Workflow concluded with: failure
     Suggestion: Check workflow logs for details

   Logs: https://github.com/owner/repo/actions/runs/123456789
     [l] view logs  [o] open in browser

   [esc/q] close  [c] copy script
   ```

3. **User presses 'l' to view logs:**
   ```
   Workflow Run #123456789 - Failed Steps

   Job: verify-ci-status (failure)
     ✗ Check CI Status: failure

   To view full logs locally:
     gh run view 123456789 --log

   [↑↓] scroll  [o] open in browser  [esc/q] close
   ```

4. **User presses 'o' to open in browser:**
   - Browser opens to GitHub Actions run page
   - User can view full logs, rerun failed jobs, etc.

## Benefits

1. **Actionable errors**: Users immediately see what failed and why
2. **Quick access to logs**: One keypress to view failure details
3. **Context preservation**: Run URLs saved in error state
4. **Reduced friction**: No need to search for failed run in GitHub UI
5. **Better debugging**: Structured errors help identify root causes
6. **Graceful degradation**: Falls back to error string if rich info unavailable

## Future Enhancements

1. **Real-time log streaming**: Stream logs as chain executes
2. **Smart error parsing**: Extract specific error messages from logs
3. **Automatic retry**: Offer to retry failed steps after issues resolved
4. **Error notifications**: Desktop notifications for failures
5. **Log caching**: Cache logs locally for offline viewing
6. **Diff view**: Show what changed that might have caused failure
