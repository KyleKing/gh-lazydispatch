# Implementation Guide: Commitizen Bump Chain & Enhanced Error Alerting

## Overview

This guide covers implementing:
1. A Chain that conditionally runs `commitizen bump` after CI passes
2. Enhanced error alerting with log access for Chain failures

## Quick Start

### Adding the Commitizen Bump Chain

1. **Create workflow files in your repository:**
   ```bash
   # Copy example workflows
   mkdir -p .github/workflows

   # See docs/chain-examples.md for full workflow definitions
   # Create: .github/workflows/ci-gate.yml
   # Create: .github/workflows/version-bump.yml
   ```

2. **Add chain definition to `.github/lazydispatch.yml`:**
   ```yaml
   version: 2
   chains:
     release-bump:
       description: Bump version with commitizen after CI passes on main
       variables:
         - name: target_branch
           type: string
           description: Branch to verify CI status
           default: main
           required: true
       steps:
         - workflow: ci-gate.yml
           wait_for: success
           on_failure: abort
           inputs:
             ref: '{{ var.target_branch }}'
         - workflow: version-bump.yml
           wait_for: success
           inputs:
             bump_type: ''
             push: 'true'
   ```

3. **Test the chain:**
   ```bash
   lazydispatch
   # Select "release-bump" chain
   # Verify CI status check behavior
   # Confirm version bump execution
   ```

## Implementation Phases

### Phase 1: Commitizen Bump Chain (No Code Changes)

**Goal:** Add working chain using existing lazydispatch features.

**Steps:**
1. Create `ci-gate.yml` workflow (see chain-examples.md)
2. Create `version-bump.yml` workflow (see chain-examples.md)
3. Add `release-bump` chain to `.github/lazydispatch.yml`
4. Test with both passing and failing CI scenarios

**Testing:**
```bash
# Test 1: CI passing
git checkout main
git pull
# Ensure CI is green
lazydispatch
# Select release-bump chain
# Verify: ci-gate passes, version bumps

# Test 2: CI failing
# Create branch with failing test
git checkout -b test-ci-gate
# Break a test
git commit -am "test: break test for ci-gate"
git push -u origin test-ci-gate
lazydispatch
# Select release-bump chain, set target_branch=test-ci-gate
# Verify: ci-gate fails, version-bump skipped

# Test 3: Specific checks
lazydispatch
# Select release-bump chain
# Set required_checks="tests,lint"
# Verify: only specified checks are verified
```

**Deliverable:** Working chain with CI gating, no code changes needed.

---

### Phase 2: Enhanced Error Types (Code Changes Required)

**Goal:** Create structured error types with context.

**Files to Create:**
- `internal/errors/chain_errors.go`

**Files to Modify:**
- `internal/chain/executor.go`

**Implementation Steps:**

1. **Create error types:**
   ```bash
   # Create new file
   touch internal/errors/chain_errors.go
   ```

   Add structured error types:
   - `StepExecutionError` (workflow failed)
   - `StepDispatchError` (dispatch failed)
   - `InterpolationError` (template failed)

   See chain-failure-alerting.md for complete implementation.

2. **Update executor.go:**

   **In `runStep()` around line 240:**
   ```go
   inputs, err := InterpolateInputs(step.Inputs, ctx)
   if err != nil {
       return nil, &errors.InterpolationError{
           StepIndex:  idx,
           Template:   fmt.Sprintf("%v", step.Inputs),
           Reason:     err.Error(),
           Suggestion: "Check that referenced variables exist",
       }
   }
   ```

   **After dispatch, around line 251:**
   ```go
   runID, err := runner.ExecuteAndGetRunID(cfg, e.client)
   if err != nil {
       return nil, &errors.StepDispatchError{
           StepIndex:    idx,
           Workflow:     step.Workflow,
           ErrorMessage: err.Error(),
           Suggestion:   "Verify workflow file exists",
       }
   }
   ```

   **After wait completes, around line 277:**
   ```go
   if conclusion != github.ConclusionSuccess && step.WaitFor == config.WaitSuccess {
       status = StepFailed
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
   ```

3. **Add tests:**
   ```bash
   # Add test file
   touch internal/errors/chain_errors_test.go
   ```

   Test error creation, messages, and type assertions.

4. **Run existing tests:**
   ```bash
   mise run ci
   ```

**Testing:**
```bash
# Test structured errors are created
go test ./internal/chain/... -v

# Test error messages contain expected fields
go test ./internal/errors/... -v

# Integration test: trigger failing chain
lazydispatch
# Select chain with failing step
# Verify error is StepExecutionError type (via logging or debugging)
```

**Deliverable:** Rich error types with context and suggestions.

---

### Phase 3: Enhanced Error Display (Code Changes Required)

**Goal:** Improve error display in ChainStatusModal.

**Files to Modify:**
- `internal/ui/styles.go`
- `internal/ui/modal/chain_status.go`

**Implementation Steps:**

1. **Add error styles to styles.go:**
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

2. **Add helper methods to chain_status.go:**
   ```go
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

3. **Update View() method (replace lines 159-163):**
   ```go
   if m.state.Error != nil {
       s.WriteString("\n")
       s.WriteString(ui.ErrorTitleStyle.Render("Error Details:"))
       s.WriteString("\n")

       detailedMsg := m.GetDetailedError()
       for _, line := range strings.Split(detailedMsg, "\n") {
           s.WriteString(ui.ErrorStyle.Render("  " + line))
           s.WriteString("\n")
       }

       if runURL := m.GetFailedStepRunURL(); runURL != "" {
           s.WriteString("\n")
           s.WriteString(ui.SubtitleStyle.Render("Logs: "))
           s.WriteString(ui.URLStyle.Render(runURL))
           s.WriteString("\n")
       }
   }
   ```

4. **Run tests:**
   ```bash
   mise run ci
   ```

**Testing:**
```bash
# Visual test: trigger failing chain
lazydispatch
# Select chain with failing step
# Verify:
# - "Error Details:" header in red/bold
# - Multi-line error message displayed
# - Workflow URL shown if available
# - URL is underlined and distinct color

# Snapshot test (optional)
# Take screenshots of error display for documentation
```

**Deliverable:** Improved error display with URLs and context.

---

### Phase 4: Log Access (Code Changes Required)

**Goal:** Add ability to view logs and open browser from error modal.

**Files to Create:**
- `internal/github/logs.go`
- `internal/ui/modal/logs_viewer.go`

**Files to Modify:**
- `internal/ui/modal/chain_status.go`
- `internal/app/app.go`
- `internal/app/handlers.go`

**Implementation Steps:**

1. **Create logs.go:**
   ```go
   // GetWorkflowRunLogsSummary fetches a summary of failed jobs
   func (c *Client) GetWorkflowRunLogsSummary(runID int64) (string, error) {
       jobs, err := c.GetWorkflowRunJobs(runID)
       if err != nil {
           return "", err
       }

       var summary strings.Builder
       summary.WriteString(fmt.Sprintf("Run #%d Summary\n\n", runID))

       for _, job := range jobs {
           if job.Conclusion != ConclusionSuccess {
               summary.WriteString(fmt.Sprintf("Job: %s (%s)\n",
                   job.Name, job.Conclusion))

               for _, step := range job.Steps {
                   if step.Conclusion != ConclusionSuccess &&
                      step.Conclusion != "" {
                       summary.WriteString(fmt.Sprintf("  ✗ %s: %s\n",
                           step.Name, step.Conclusion))
                   }
               }
           }
       }

       return summary.String(), nil
   }
   ```

2. **Update ChainStatusModal keymap:**
   ```go
   type chainStatusKeyMap struct {
       Close    key.Binding
       Stop     key.Binding
       Copy     key.Binding
       OpenURL  key.Binding  // NEW: 'o' key
   }
   ```

3. **Add OpenURL handler in chain_status.go:**
   ```go
   case key.Matches(msg, m.keys.OpenURL):
       if runURL := m.GetFailedStepRunURL(); runURL != "" {
           return m, tea.Exec(
               exec.Command("open", runURL),
               func(err error) tea.Msg { return nil },
           )
       }
   ```

4. **Update help text in View():**
   ```go
   if runURL := m.GetFailedStepRunURL(); runURL != "" {
       s.WriteString("\n")
       s.WriteString(ui.HelpStyle.Render("[o] open logs in browser"))
       s.WriteString("\n")
   }
   ```

5. **Add tests:**
   ```bash
   touch internal/github/logs_test.go
   ```

**Testing:**
```bash
# Test log summary fetching
go test ./internal/github/... -v -run TestGetWorkflowRunLogsSummary

# Integration test
lazydispatch
# Trigger failing chain
# Press 'o' when error displayed
# Verify: browser opens to correct run URL

# Cross-platform test
# macOS: uses 'open'
# Linux: may need 'xdg-open'
# Windows: may need 'start'
```

**Deliverable:** Ability to open logs in browser from error modal.

---

### Phase 5: Log Viewer Modal (Optional Enhancement)

**Goal:** Display log summary within TUI.

**Implementation Steps:**

1. **Create logs_viewer.go modal**
2. **Add FetchLogsMsg and LogsFetchedMsg types**
3. **Wire up message handling**
4. **Add 'l' key to chain_status.go to trigger fetch**

See chain-failure-alerting.md for complete implementation.

**Testing:**
```bash
# Test modal display
go test ./internal/ui/modal/... -v -run TestLogsViewerModal

# Integration test
lazydispatch
# Trigger failing chain
# Press 'l' when error displayed
# Verify: logs modal appears with failed step details
# Verify: can scroll through logs
# Verify: 'o' opens browser, 'esc' closes modal
```

**Deliverable:** In-TUI log viewer for quick failure diagnosis.

---

## Testing Strategy

### Unit Tests

**Priority: High**

```bash
# Test error types
go test ./internal/errors/... -v -cover

# Test executor error creation
go test ./internal/chain/... -v -cover

# Test GitHub client log methods
go test ./internal/github/... -v -cover

# Test modal rendering (can use golden files)
go test ./internal/ui/modal/... -v -cover
```

### Integration Tests

**Priority: Medium**

```bash
# Test end-to-end chain execution with errors
# Use testdata workflows that intentionally fail
go test ./internal/... -v -run TestChainExecutor

# Test error propagation through app
# Mock GitHub client to return failing runs
```

### Manual Testing Checklist

**Priority: High**

- [ ] CI gate passes when all checks green
- [ ] CI gate fails when checks pending
- [ ] CI gate fails when checks failed
- [ ] Error displays workflow name and step
- [ ] Error shows actionable suggestion
- [ ] Workflow URL appears in error
- [ ] 'o' key opens browser to correct URL
- [ ] Error styling matches theme (Catppuccin)
- [ ] Works on macOS, Linux, Windows
- [ ] Errors persist after chain completes
- [ ] Can copy script even when failed
- [ ] Stop (C-c) works during chain execution

### Regression Tests

**Priority: High**

```bash
# Ensure existing chains still work
mise run ci

# Test all existing examples
lazydispatch
# Run each chain in testdata/lazydispatch.yml
# Verify no regressions in display or behavior
```

---

## Configuration Examples

### Minimal Configuration

```yaml
version: 2
chains:
  release-bump:
    description: Bump version after CI passes
    steps:
      - workflow: ci-gate.yml
        wait_for: success
        on_failure: abort
        inputs:
          ref: main
      - workflow: version-bump.yml
        inputs:
          push: 'true'
```

### Full Configuration

```yaml
version: 2
chains:
  release-bump:
    description: Bump version with commitizen after CI passes on main
    variables:
      - name: target_branch
        type: string
        description: Branch to verify CI status
        default: main
        required: true

      - name: required_checks
        type: string
        description: Required check names (comma-separated, empty for all)
        default: ''
        required: false

      - name: bump_type
        type: choice
        description: Version bump type (empty for auto-detect)
        options: ['', 'patch', 'minor', 'major']
        default: ''

      - name: prerelease
        type: string
        description: Prerelease identifier (e.g., alpha, beta, rc)
        default: ''

      - name: push
        type: boolean
        description: Push changes and tags to remote
        default: 'true'

    steps:
      - workflow: ci-gate.yml
        wait_for: success
        on_failure: abort
        inputs:
          ref: '{{ var.target_branch }}'
          required_checks: '{{ var.required_checks }}'

      - workflow: version-bump.yml
        wait_for: success
        on_failure: abort
        inputs:
          bump_type: '{{ var.bump_type }}'
          prerelease: '{{ var.prerelease }}'
          push: '{{ var.push }}'
```

### Multi-Environment Configuration

```yaml
version: 2
chains:
  release-staging:
    description: Release to staging after CI passes
    steps:
      - workflow: ci-gate.yml
        wait_for: success
        inputs:
          ref: develop
      - workflow: version-bump.yml
        inputs:
          bump_type: patch
          prerelease: rc
          push: 'true'

  release-production:
    description: Release to production after CI passes on main
    steps:
      - workflow: ci-gate.yml
        wait_for: success
        inputs:
          ref: main
          required_checks: 'tests,lint,security-scan'
      - workflow: version-bump.yml
        inputs:
          bump_type: ''
          push: 'true'
```

---

## Troubleshooting

### CI Gate Always Fails

**Symptom:** CI gate reports no checks found or incomplete checks.

**Causes:**
1. Workflow uses branch without CI configured
2. CI hasn't run yet on target branch
3. `required_checks` names don't match actual check names

**Solutions:**
```bash
# Check what checks exist for a commit
gh api repos/:owner/:repo/commits/:sha/check-runs

# List all status checks
gh api repos/:owner/:repo/commits/:sha/status

# Verify check names match
# In ci-gate.yml, add debug output:
echo "Required: ${{ inputs.required_checks }}"
echo "Found checks:"
echo "$allChecks" | jq -r '.[].name'
```

### Error URL Not Showing

**Symptom:** Error appears but no workflow URL displayed.

**Causes:**
1. Error type is not `StepExecutionError`
2. `GetWorkflowRun()` failed to fetch URL
3. Modal not checking for URL correctly

**Debug:**
```go
// In executor.go, add logging
log.Printf("Error type: %T", err)
log.Printf("Run URL: %s", run.HTMLURL)

// In chain_status.go, add debug output
log.Printf("Failed step URL: %s", m.GetFailedStepRunURL())
```

### Browser Won't Open

**Symptom:** Pressing 'o' does nothing or errors.

**Causes:**
1. `open` command not available (Linux/Windows)
2. No URL to open
3. Permission issues

**Solutions:**
```go
// Make cross-platform
var cmd *exec.Cmd
switch runtime.GOOS {
case "darwin":
    cmd = exec.Command("open", url)
case "linux":
    cmd = exec.Command("xdg-open", url)
case "windows":
    cmd = exec.Command("cmd", "/c", "start", url)
}
```

### Tests Failing After Changes

**Symptom:** Tests pass locally but fail in CI.

**Common Issues:**
1. Race conditions in concurrent tests
2. Mocked GitHub client returning wrong types
3. Snapshot tests need updating

**Debug:**
```bash
# Run with race detector
go test -race ./...

# Run specific test with verbose output
go test ./internal/chain/... -v -run TestExecutorErrors

# Update golden files if needed
go test ./internal/ui/modal/... -update
```

---

## Performance Considerations

### Chain Execution

- **Polling interval:** 5 seconds (watcher.PollInterval)
- **API calls:** 1 per step per interval during wait
- **Buffered channels:** 10 for chains, 100 for watchers

**Optimization:**
```go
// For faster feedback, reduce poll interval
const PollInterval = 3 * time.Second  // in watcher/watcher.go
```

### Log Fetching

- **Summary fetch:** 1-2 API calls (run + jobs)
- **Full logs:** Downloads zip file (can be large)

**Recommendation:** Use summary by default, link to full logs.

---

## Documentation Updates

After implementation, update:

1. **README.md:**
   - Add section on Chains with CI gating
   - Link to chain-examples.md

2. **AGENTS.md:**
   - Note error handling patterns
   - Document structured error types

3. **User Guide:**
   - Add screenshots of error display
   - Explain keyboard shortcuts for log access

---

## Future Enhancements

### Near-term (Next 1-2 releases)

1. **Log streaming:** Real-time log display during execution
2. **Retry mechanism:** Automatically retry failed steps
3. **Error notifications:** Desktop notifications for failures
4. **Log search:** Search within logs from TUI

### Long-term (3+ releases)

1. **AI error analysis:** Use LLM to suggest fixes
2. **Dependency detection:** Warn about breaking changes
3. **Rollback support:** Automatic rollback on failure
4. **Multi-repo chains:** Coordinate across repositories
5. **Approval steps:** Human-in-loop for critical steps

---

## Resources

- **Chain Examples:** docs/chain-examples.md
- **Error Alerting Design:** docs/chain-failure-alerting.md
- **GitHub Actions API:** https://docs.github.com/en/rest/actions
- **Commitizen:** https://commitizen-tools.github.io/commitizen/
- **Bubbletea:** https://github.com/charmbracelet/bubbletea
