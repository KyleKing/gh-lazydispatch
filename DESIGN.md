# Design

## Architecture

### Project Context

- **Module**: `github.com/kyleking/gh-lazydispatch`
- **Type**: CLI application (GH CLI extension + standalone)
- **Description**: Interactive GitHub Actions workflow dispatcher TUI with fuzzy selection, input configuration, and frecency-based history

### Package Structure

```
gh-lazydispatch/
├── cmd/gh-lazydispatch/  # CLI entry point
├── internal/
│   ├── app/          # Main Bubbletea application
│   ├── browser/      # Browser interaction
│   ├── chain/        # Workflow chains (multi-step execution)
│   ├── config/       # YAML configuration parsing
│   ├── demo/         # Demo mode
│   ├── errors/       # Domain-specific error types
│   ├── exec/         # Command execution (with mocks)
│   ├── frecency/     # Frequency/recency history tracking
│   ├── git/          # Git branch operations
│   ├── github/       # GitHub API client
│   ├── logs/         # Log fetching, filtering, caching, streaming
│   ├── rule/         # Dispatch rules
│   ├── runner/       # Workflow execution orchestration
│   ├── testutil/     # Test fixtures and mocks
│   ├── ui/           # TUI rendering
│   │   ├── styles.go # Lipgloss styling
│   │   ├── theme/    # Catppuccin themes
│   │   ├── modal/    # Modal dialogs
│   │   └── panes/    # Main view panes
│   ├── validation/   # Input validation
│   ├── watcher/      # File and workflow watchers
│   └── workflow/     # Workflow discovery and parsing
├── testdata/         # Test data and fixtures
└── go.mod
```

## Patterns

### Bubbletea Patterns

**Channel Communication:**

- Use buffered channels for async updates (buffer size: 10-100)
- Never silently drop messages - log warnings and surface errors
- Prefer `select` with `default` only when loss is acceptable AND logged

**Error Surfacing:**

- Errors from async operations must reach the UI
- Use `RunUpdate.Error` pattern for watcher errors
- Display actionable error messages with resolution hints

**Message Types:**

- Define specific Msg types for each async operation
- Pattern: `type XxxResultMsg struct { Value T; Err error }`

**UI Architecture:**

- Four-pane layout: status bar, workflows (left), tabbed panel (right), config (bottom)
- Modals: centered overlays with Esc to cancel, Enter to confirm
- Keyboard-first: 1-9/0 shortcuts, Tab for pane switching, h/l for tab navigation
- Visual feedback: status icons (o/\*/+/x/-), dimmed defaults, validation errors
- See `UX.md` for complete layout, shortcuts, and interaction patterns

### Constants

- Define numeric constants for magic numbers
- Use shared constants across packages (e.g., `watcher.PollInterval`)

## Design decisions

### Input type mapping

| `workflow_dispatch` input type | TUI component                 |
| ------------------------------ | ----------------------------- |
| `string` (default)             | Text input                    |
| `boolean`                      | Confirm                       |
| `choice`                       | Select with options           |
| `environment`                  | Select with repo environments |

A `workflow_dispatch` input with no `type` is a `string`. GitHub coerces booleans to strings in the `github.event.inputs` context but preserves them in `inputs`.

### Sequential prompts over a dashboard

Configuration happens one decision at a time through the modal stack rather than in a single form showing every field at once. A full-screen dashboard needs more state management without a clear win for the primary flow, and the stack gives lazygit-style Esc-to-go-back for free.

### Frecency scoring

`internal/frecency` multiplies run count by a recency weight: 4.0 under an hour, 2.0 under a day, 1.0 under a week, 0.5 beyond that. History is JSON at `$XDG_CACHE_HOME/lazydispatch/history.json` (falling back to `~/.cache`). SQLite would query large histories faster but adds a CGO dependency, and JSON handles hundreds of entries with no measurable cost.

## Testing

### Coverage Targets

- Critical packages (logs, runner, workflow): >90%
- Core packages (frecency, validation, git): >80%
- UI packages (panes, modal): >70%
- Utilities and helpers: >60%

### Mock Patterns

- Use `MockExecutor` for gh CLI commands (`internal/exec/mock_executor.go`)
- Use `MockGitHubClient` for API calls (`internal/testutil/mocks.go`)
- Create package-local mocks for interfaces when needed

### Test Safety

- NEVER use `exec.NewRealExecutor()` in tests - always use `exec.NewMockExecutor()`
- Runtime safety check panics on mutation commands during tests (gh workflow run, gh pr create, etc.)
- Always inject mocks: `runner.SetExecutor(mockExec)` or use `...WithExecutor` functions

### Fixture Patterns

- Store test data in `testdata/` (workflows, logs, configs)
- Generate large datasets programmatically (see `internal/testutil/fixtures.go`)
- Use `t.TempDir()` for temporary file operations

### Async/Channel Testing

- Use buffered channels with `select` and timeouts
- Always drain channels or use `context.WithTimeout`
- Verify channel closure after Stop()
