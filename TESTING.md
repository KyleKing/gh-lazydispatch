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

**Purpose**: Test complete workflows including GitHub API calls and gh CLI execution.

**Characteristics**:
- Test realistic scenarios (success, failure, warnings, errors)
- Use mocked GitHub API responses (httpmock)
- Use mocked gh CLI execution (CommandExecutor interface)
- Use realistic test fixtures from `testdata/logs/`
- Run in CI without requiring real GitHub credentials

**Scenarios Covered**:
- ✅ Successful workflow run
- ✅ Failed workflow run with error detection
- ✅ Workflow with warnings
- ✅ gh CLI command failures
- ✅ GitHub API errors
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

### GitHub API Mocking

Uses `httpmock` to intercept HTTP requests:

```go
httpmock.Activate()
defer httpmock.DeactivateAndReset()

httpmock.RegisterResponder("GET", "https://api.github.com/repos/owner/repo/actions/runs/123/jobs",
    httpmock.NewJsonResponderOrPanic(200, github.JobsResponse{
        Jobs: []github.Job{...},
    }))
```

**Limitation**: The `go-gh` library creates its own HTTP client that doesn't use `http.DefaultTransport`, which prevents `httpmock` from intercepting all requests. As a result, some API-level integration tests may require alternative approaches or be skipped.

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
2. Set up both httpmock and MockExecutor
3. Test the full flow from API call to log parsing

```go
func TestIntegration_MyScenario(t *testing.T) {
    // Setup httpmock
    httpmock.Activate()
    defer httpmock.DeactivateAndReset()
    httpmock.RegisterResponder("GET", "...", ...)

    // Setup mock executor
    mockExec := exec.NewMockExecutor()
    logOutput := loadFixture(t, "my_fixture.txt")
    mockExec.AddGHRunView(runID, jobID, logOutput)

    // Create client and fetcher
    client, _ := github.NewClient("owner/repo")
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
