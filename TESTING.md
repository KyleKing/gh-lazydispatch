# Testing Guide

This document describes the testing strategy for gh-lazydispatch, including how to run tests and how the mock infrastructure works.

## Overview

The project uses a multi-layered testing approach:

1. **Unit Tests** - Fast, isolated tests using in-memory mocks
2. **Integration Tests** - Tests that exercise the full flow with mocked external dependencies
3. **CI Tests** - All tests run automatically in GitHub Actions

## Running Tests

```bash
# Run all tests with coverage
mise run test

# Run only integration tests
mise run test:integration

# Run specific test
go test -v -run TestIntegration_SuccessfulWorkflowRun ./internal/logs

# View coverage report
mise run test:view-coverage

# Run CI checks (tests + build)
mise run ci
```

## Test Categories

### Unit Tests

Located alongside the code they test (e.g., `watcher_test.go`, `executor_test.go`).

**Purpose**: Test individual components in isolation with minimal dependencies.

**Characteristics**:
- Fast execution (milliseconds)
- Use simple in-memory mocks implementing interfaces
- Focus on business logic and edge cases
- No network I/O or external dependencies

**Example**:
```go
type mockGitHubClient struct {
    runs map[int64]*github.WorkflowRun
    err  error
}

func (m *mockGitHubClient) GetWorkflowRun(runID int64) (*github.WorkflowRun, error) {
    if m.err != nil {
        return nil, m.err
    }
    return m.runs[runID], nil
}
```

### Integration Tests

Located in `internal/logs/integration_test.go`.

**Purpose**: Test complete workflows including all GitHub interactions via gh CLI.

**Characteristics**:
- Test realistic scenarios (success, failure, warnings, errors)
- Use mocked gh CLI execution (CommandExecutor interface) for ALL GitHub interactions
- Use realistic test fixtures from `testdata/logs/`
- Run in CI without requiring real GitHub credentials
- Unified mocking strategy: all GitHub operations (API calls and log fetching) use the same CommandExecutor mock

**Scenarios Covered**:
- ✅ Successful workflow run
- ✅ Failed workflow run with error detection
- ✅ Workflow with warnings
- ✅ gh CLI command failures (for both API calls and log fetching)
- ✅ GitHub API errors (via gh api command failures)
- ✅ gh CLI availability checking

## Mock Infrastructure

### CommandExecutor Interface

The `internal/exec` package provides an abstraction for command execution:

```go
type CommandExecutor interface {
    Execute(name string, args ...string) (stdout, stderr string, err error)
}
```

**RealExecutor**: Executes actual system commands (production use)
**MockExecutor**: Simulates command execution for testing

### Safety Mechanisms

To prevent accidental mutation of real GitHub resources during tests, multiple safety layers are in place:

#### 1. Runtime Safety Check (Primary Protection)

`exec.RealExecutor` includes a runtime check that panics if mutation commands are executed during tests:

```go
// This will panic during tests:
executor := exec.NewRealExecutor()
executor.Execute("gh", "workflow", "run", "test.yml") // PANIC!

// Instead, always use mocks in tests:
mockExec := exec.NewMockExecutor()
mockExec.AddCommand("gh", []string{"workflow", "run", "test.yml"}, "", "", nil)
```

Blocked mutation commands include:
- `gh workflow run` - Dispatches workflows
- `gh issue create/edit/close` - Issue mutations
- `gh pr create/merge/close` - PR mutations
- `gh run cancel/rerun` - Workflow run mutations
- And many others (see `internal/exec/executor.go:isMutationCommand`)

Read-only commands are allowed:
- `gh api` - GitHub API read calls
- `gh run view/list/watch` - Workflow run inspection
- Non-gh commands

#### 2. Static Analysis (Secondary Protection)

Run `./scripts/check-test-safety.sh` to detect unsafe patterns:
- Direct `exec.NewRealExecutor()` usage in test files
- `runner.Execute*` calls without mock setup

Add to CI to catch issues early:
```yaml
- name: Check test safety
  run: ./scripts/check-test-safety.sh
```

#### 3. Test Design (Best Practice)

Always use one of these patterns in tests:

```go
// Pattern 1: SetExecutor for integration tests
mockExec := exec.NewMockExecutor()
runner.SetExecutor(mockExec)
defer runner.SetExecutor(nil)

// Pattern 2: ...WithExecutor for unit tests
mockExec := &mockCommandExecutor{}
runner.ExecuteWithExecutor(cfg, mockExec)
```

### Runner Package Mocking

The `internal/runner` package provides two ways to inject mocks for testing:

1. **SetExecutor (for integration tests)**: Injects `exec.CommandExecutor` globally
   ```go
   mockExec := exec.NewMockExecutor()
   mockExec.AddCommand("gh", []string{"workflow", "run", "test.yml"}, "", "", nil)
   runner.SetExecutor(mockExec)
   defer runner.SetExecutor(nil) // Reset after test
   ```

2. **...WithExecutor functions (for unit tests)**: Accept `runner.CommandExecutor` as parameter
   ```go
   mockExec := &mockCommandExecutor{} // Test-specific mock
   err := runner.ExecuteWithExecutor(cfg, mockExec)
   ```

This dual approach ensures:
- No real `gh workflow run` commands execute during tests
- Chain executor tests can use `SetExecutor` to prevent accidental mutations
- Runner package tests can use focused mocks with `...WithExecutor` functions

### Mock Executor Usage

```go
mockExec := exec.NewMockExecutor()

// Add expected gh CLI response
mockExec.AddGHRunView(12345, 67890, logOutput)

// Add gh CLI error
mockExec.AddGHRunViewError(12345, 67890, "auth failed", fmt.Errorf("exit 1"))

// Create fetcher with mock
fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)
```

### Test Fixtures

Located in `testdata/logs/`:
- `successful_run.txt` - Complete successful workflow logs
- `failed_run.txt` - Workflow with installation failure
- `run_with_warnings.txt` - Workflow with linter warnings

Fixtures contain realistic GitHub Actions log output with:
- Step boundaries (`##[group]`, `##[endgroup]`)
- Error patterns (`ERROR:`, `##[error]`)
- Warning patterns (`WARNING:`)
- Actual command output from pip, pytest, etc.

### GitHub API Mocking via gh CLI

All GitHub API interactions are mocked through the CommandExecutor interface using `gh api` and `gh workflow run` commands:

```go
mockExec := exec.NewMockExecutor()

// Mock gh api response for GetWorkflowRunJobs
jobsResp := github.JobsResponse{
    Jobs: []github.Job{...},
}
jobsJSON, _ := json.Marshal(jobsResp)
mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/123/jobs"}, string(jobsJSON), "", nil)

// Mock gh workflow run for dispatching workflows
mockExec.AddCommand("gh", []string{"workflow", "run", "test.yml", "--ref", "main"}, "", "", nil)

// Mock gh run view for log fetching
logOutput := loadFixture(t, "successful_run.txt")
mockExec.AddGHRunView(runID, jobID, logOutput)
```

**Benefits of unified gh CLI approach**:
- Single mocking point for all GitHub interactions
- Consistent authentication handling
- Prevents accidental mutation of real GitHub resources in tests
- Simpler test setup
- No HTTP transport layer mocking needed

## Writing New Tests

### Adding a Unit Test

1. Create `*_test.go` file alongside the code
2. Define mock implementations of interfaces
3. Test individual functions/methods in isolation

```go
func TestMyFunction(t *testing.T) {
    mockClient := &mockGitHubClient{
        runs: map[int64]*github.WorkflowRun{
            123: {ID: 123, Status: github.StatusCompleted},
        },
    }

    result := MyFunction(mockClient, 123)
    // assertions...
}
```

### Adding an Integration Test

1. Create test fixture in `testdata/logs/` if needed
2. Set up MockExecutor with both gh api and gh run view mocks
3. Test the full flow from API call to log parsing

```go
func TestIntegration_MyScenario(t *testing.T) {
    runID := int64(12345)
    jobID := int64(67890)

    // Setup mock executor
    mockExec := exec.NewMockExecutor()

    // Mock gh api response for GetWorkflowRunJobs
    jobsResp := github.JobsResponse{
        Jobs: []github.Job{
            {
                ID:         jobID,
                Name:       "build",
                Status:     github.StatusCompleted,
                Conclusion: github.ConclusionSuccess,
                Steps: []github.Step{
                    {Name: "Run tests", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
                },
            },
        },
    }
    jobsJSON, _ := json.Marshal(jobsResp)
    mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/12345/jobs"}, string(jobsJSON), "", nil)

    // Mock gh run view for log fetching
    logOutput := loadFixture(t, "my_fixture.txt")
    mockExec.AddGHRunView(runID, jobID, logOutput)

    // Create client and fetcher with mock executor
    client, _ := github.NewClientWithExecutor("owner/repo", mockExec)
    fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

    // Execute and assert
    stepLogs, err := fetcher.FetchStepLogsReal(runID, "workflow.yml")
    // assertions...
}
```

### Creating Test Fixtures

1. Run a real GitHub workflow
2. Capture the output of `gh run view <run-id> --log`
3. Save to `testdata/logs/descriptive_name.txt`
4. Ensure it includes relevant log patterns (errors, warnings, etc.)

## Test Structure Best Practices

Follow the table-driven test pattern from AGENTS.md:

```go
tests := []struct {
    name        string
    setupMock   func(*exec.MockExecutor)
    expectError bool
}{
    {
        name: "successful execution",
        setupMock: func(m *exec.MockExecutor) {
            m.AddCommand("gh", []string{"--version"}, "gh version 2.40.0", "", nil)
        },
        expectError: false,
    },
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test implementation
    })
}
```

## CI Integration

Tests run automatically on:
- Every push to a branch
- Every pull request
- Manual workflow dispatch

CI configuration: `.github/workflows/test.yml`

## Debugging Failed Tests

### Verbose Output
```bash
go test -v ./internal/logs/... -run TestIntegration_FailedWorkflowRun
```

### Check Mock Executor History
```go
t.Logf("Executed commands: %+v", mockExec.ExecutedCommands)
```

### Inspect Fixture Content
```bash
cat testdata/logs/failed_run.txt
```

### Test Single Scenario
```bash
go test -v -run TestIntegration_SuccessfulWorkflowRun ./internal/logs
```

## Future Enhancements

Potential testing improvements:

1. **Record/Replay Mode**: Record real API interactions for playback in tests
2. **End-to-End Tests**: Test full TUI with simulated user input
3. **Performance Benchmarks**: `go test -bench` for critical paths
4. **Contract Tests**: Verify assumptions about GitHub API responses
5. **Property-Based Tests**: Using `testing/quick` for edge case discovery

## Related Documentation

- [AGENTS.md](./AGENTS.md) - AI agent guidelines and project patterns
- [IMPLEMENTATION_CHECKLIST.md](./IMPLEMENTATION_CHECKLIST.md) - Feature implementation status
- [docs/test-safety-example.md](./docs/test-safety-example.md) - Detailed examples of test safety violations and fixes
