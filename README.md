# lazydispatch

![.github/assets/demo.gif](https://raw.githubusercontent.com/kyleking/lazydispatch/main/.github/assets/demo.gif)

Interactive GitHub Actions workflow dispatcher TUI with fuzzy selection, input configuration, and frecency-based history.

## Features

- Fuzzy search for workflow selection
- Interactive input configuration for workflow_dispatch inputs
- Branch selection
- Watch mode for real-time workflow run updates
- Frecency-based workflow history tracking
- Theme support (Catppuccin)
- Command preview before execution

## See Also

[gh-dispatch](https://github.com/mdb/gh-dispatch) is a CLI-based alternative that supports both `workflow_dispatch` and `repository_dispatch` with JSON payloads via command-line flags. Use gh-dispatch for scripting, CI integration, or repository_dispatch events; use lazydispatch for interactive exploration, frecency-based history, and guided input configuration.

Other alternatives:

- [chrisgavin/gh-dispatch](https://github.com/chrisgavin/gh-dispatch) - Interactive CLI for dispatching workflows with progress tracking
- [gh workflow run](https://cli.github.com/manual/gh_workflow_run) - Built-in `gh` command with basic interactive prompts
- [nektos/act](https://github.com/nektos/act) - Run GitHub Actions locally in Docker (different use case: local testing vs remote dispatch)

## Installation

### As a GitHub CLI Extension (Recommended)

```bash
gh extension install KyleKing/lazydispatch
```

Then run with:

```bash
gh lazydispatch
```

### Standalone Binary

```bash
go install github.com/kyleking/gh-lazydispatch@latest
```

Or build from source:

```bash
git clone https://github.com/kyleking/gh-lazydispatch
cd lazydispatch
go build
```

## Usage

Navigate to a directory with a Git repository containing GitHub Actions workflows:

```bash
cd your-project

# If installed as gh extension:
gh lazydispatch

# If installed as standalone:
lazydispatch
```

The tool will discover all workflows with `workflow_dispatch` triggers and present them in an interactive TUI.

### Keyboard Shortcuts

- `Tab` / `Shift+Tab` - Switch between panes
- `↑/k`, `↓/j` - Navigate within pane
- `Enter` - Select / Execute workflow
- `b` - Select branch
- `w` - Toggle watch mode
- `1-9` - Edit input by number
- `?` - Show help
- `q`, `Ctrl+C` - Quit

### Environment Variables

- `CATPPUCCIN_THEME` - Override theme (latte/macchiato)

## Recording the Demo

Generate the demo GIF using VHS:

```bash
vhs < .github/assets/demo.tape
```

## Maintenance

### Updating Dependencies

Update Go version, dependencies, and GitHub Actions:

```bash
# Update Go version in go.mod (check https://go.dev/dl/ for latest)
# Then update dependencies
go get -u ./... && go mod tidy && go test ./...

# Update GitHub Actions in .github/workflows/*.yml
# Check for latest versions at:
# - https://github.com/actions/checkout/releases
# - https://github.com/actions/setup-go/releases
# - https://github.com/golangci/golangci-lint/releases
# - https://github.com/goreleaser/goreleaser-action/releases
```
