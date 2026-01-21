# AI Agent Guidelines for lazydispatch

This document provides guidelines for AI coding assistants working on this Go project.

## Project Context

- **Module**: `github.com/kyleking/gh-lazydispatch`
- **Type**: CLI application
- **Description**: Interactive GitHub Actions workflow dispatcher TUI with fuzzy selection, input configuration, and frecency-based history

## Code Organization

### Package Structure

```
lazydispatch/
├── main.go           # Entry point (minimal, delegates to internal/)
├── internal/         # Private packages
│   ├── app/          # Main Bubbletea application
│   ├── workflow/     # Workflow discovery & parsing
│   ├── frecency/     # History & scoring
│   ├── ui/           # TUI components
│   │   ├── panes/    # Main view panes
│   │   ├── modal/    # Modal dialogs
│   │   ├── styles.go # Lipgloss styling
│   │   └── theme/    # Catppuccin themes
│   ├── runner/       # Workflow execution
│   ├── git/          # Git operations
│   └── validation/   # Input validation
├── testdata/         # Test fixtures
└── go.mod
```

### Package Guidelines

- One package = one purpose
- Package names: short, lowercase, no underscores (`httputil` not `http_util`)
- Avoid `util`, `common`, `misc` packages
- `internal/` prevents external imports at the compiler level

### File Organization

- Group related types, functions, and methods in the same file
- Name files after the primary type they contain (`user.go`, `user_test.go`)
- Keep `main.go` minimal

## Code Style

### Functional Composition

```go
func ValidateUser(u User) error {
    if err := validateEmail(u.Email); err != nil {
        return err
    }
    return validateAge(u.Age)
}
```

### Interfaces

- Define interfaces where they're used, not where they're implemented
- Keep interfaces small (1-3 methods)
- Avoid interface pollution

### Functional Options Pattern

```go
type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{addr: addr, timeout: 30 * time.Second}
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

## Error Handling

- Errors are values; handle them explicitly
- Return errors, don't panic (except for truly unrecoverable states)
- Add context when wrapping: `fmt.Errorf("doing something: %w", err)`
- Use `errors.Is` and `errors.As` for checking errors
- Use custom error types for domain-specific errors

## Testing

### Table-Driven Tests

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive numbers", 2, 3, 5},
        {"negative numbers", -1, -1, -2},
        {"zero", 0, 0, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Add(tt.a, tt.b)
            if got != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d", tt.a, tt.b, got, tt.expected)
            }
        })
    }
}
```

### Test Files

- Use `_test` package suffix for black-box testing
- Place test files adjacent to the code they test

### Testing Guidelines

**When to Add Tests:**
- ALWAYS add tests when implementing new features
- Tests should be written alongside code, not as an afterthought
- Aim for >80% coverage on core packages, >90% on critical paths (logs, workflow, runner)

**Test Structure:**
```
internal/package/
├── file.go           # Implementation
├── file_test.go      # Unit tests (table-driven)
├── file_bench_test.go # Benchmarks (if performance-critical)
└── integration_test.go # Integration tests (cross-component)
```

**Test Types by Layer:**

1. **Unit Tests** (most tests should be here):
   - Test individual functions/methods in isolation
   - Use table-driven tests for multiple scenarios
   - Mock external dependencies (GitHub API, gh CLI, file I/O)
   - Example: `internal/logs/filter_test.go`, `internal/frecency/store_test.go`

2. **Integration Tests** (test component interactions):
   - Test workflows that cross package boundaries
   - Use MockExecutor for gh CLI commands
   - Use test fixtures from `testdata/`
   - Example: `internal/logs/integration_test.go`, `internal/integration_test.go`

3. **Benchmark Tests** (for performance-critical code):
   - Measure time and allocations
   - Test with realistic dataset sizes (10k, 50k entries)
   - Use `b.ResetTimer()` and `b.ReportAllocs()`
   - Example: `internal/logs/filter_bench_test.go` (if added)

**Mock Patterns:**
- Use `MockExecutor` for gh CLI commands (`internal/exec/mock_executor.go`)
- Use `MockGitHubClient` for API calls (`internal/testutil/mocks.go`)
- Create package-local mocks for interfaces when needed
- Example from `streamer_test.go`:
```go
type mockGitHubClient struct{}

func (m *mockGitHubClient) GetWorkflowRun(runID int64) (*github.WorkflowRun, error) {
    return &github.WorkflowRun{ID: runID, Status: "in_progress"}, nil
}
```

**Fixture Patterns:**
- Store test data in `testdata/` (workflows, logs, configs)
- Generate large datasets programmatically (see `internal/testutil/fixtures.go`)
- Use `t.TempDir()` for temporary file operations
- Load fixtures with helper: `testutil.LoadFixture(t, "file.txt")`

**Async/Channel Testing:**
- Use buffered channels with `select` and timeouts
- Always drain channels or use `context.WithTimeout`
- Verify channel closure after Stop()
- Example pattern:
```go
select {
case update := <-streamer.Updates():
    // Verify update
case <-time.After(100 * time.Millisecond):
    t.Error("timeout waiting for update")
}
```

**Coverage Targets:**
- Critical packages (logs, runner, workflow): >90%
- Core packages (frecency, validation, git): >80%
- UI packages (panes, modal): >70%
- Utilities and helpers: >60%

**CI Integration:**
- All tests must pass: `go test ./...`
- Race detection: `go test -race ./...`
- Coverage reporting: `go test -coverprofile=coverage.out ./...`
- Benchmarks tracked: `go test -bench=. -benchmem ./...`

## Anti-Patterns to Avoid

- **Naked returns**: Always name what you're returning
- **Long functions**: If > 50 lines, consider breaking up
- **Deep nesting**: Use early returns to flatten
- **Interface pollution**: Don't define interfaces until needed
- **Ignoring errors**: `_ = doThing()` is almost always wrong
- **Global state**: Pass dependencies explicitly

## Tools

- Run `mise run ci` before committing
- Run `hk fix` to auto-fix linting issues
- Use `golangci-lint run` for detailed linting output

## Git Practices

- Do not stage, commit, or push without explicit instruction
- Use conventional commits (commitizen enforced)

## Bubbletea Patterns

### Channel Communication

- Use buffered channels for async updates (buffer size: 10-100)
- Never silently drop messages - log warnings and surface errors
- Prefer `select` with `default` only when loss is acceptable AND logged

### Error Surfacing

- Errors from async operations must reach the UI
- Use `RunUpdate.Error` pattern for watcher errors
- Display actionable error messages with resolution hints

### Message Types

- Define specific Msg types for each async operation
- Pattern: `type XxxResultMsg struct { Value T; Err error }`

## Constants

- Define numeric constants for magic numbers
- Use shared constants across packages (e.g., `watcher.PollInterval`)

## Interface Design

- Define interfaces where consumed, not where implemented
- Keep interfaces small (1-3 methods)
- Use interfaces to enable testing with mocks
