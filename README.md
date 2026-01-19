# lazydispatch

![.github/assets/demo.gif](https://raw.githubusercontent.com/kyleking/lazydispatch/main/.github/assets/demo.gif)

Interactive GitHub Actions workflow dispatcher TUI with fuzzy selection, input configuration, and frecency-based history.

![.github/assets/chains-demo.gif](https://raw.githubusercontent.com/kyleking/lazydispatch/main/.github/assets/chains-demo.gif)

## Features

- Fuzzy search for workflow selection
- Interactive input configuration for workflow_dispatch inputs
- Branch selection with frecency-based sorting
- Watch mode for real-time workflow run updates
- Frecency-based workflow history tracking
- Workflow chains for multi-step deployments
- Tabbed right panel (History, Chains, Live runs)
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

#### Navigation

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Switch between panes |
| `h` / `l` | Switch tabs (when right panel focused) |
| `j` / `k` or `Up` / `Down` | Navigate within pane |
| `Enter` | Select / Execute workflow |
| `Space` | Select workflow and jump to config |
| `Esc` | Deselect / Close modal |

#### Configuration

| Key | Action |
|-----|--------|
| `b` | Select branch |
| `w` | Toggle watch mode |
| `1-9`, `0` | Edit input by number |
| `/` | Filter inputs |
| `c` | Copy command to clipboard |
| `r` | Reset all inputs to defaults |

#### Live Runs

| Key | Action |
|-----|--------|
| `d` | Clear selected run |
| `D` | Clear all completed runs |

#### General

| Key | Action |
|-----|--------|
| `?` | Show help |
| `q`, `Ctrl+C` | Quit |

### Environment Variables

- `CATPPUCCIN_THEME` - Override theme (latte/macchiato)

## Workflow Chains

Chains let you execute multiple workflows in sequence with configurable wait conditions and failure handling. Define chains in `.github/lazydispatch.yml`:

```yaml
version: 1
chains:
  deploy-all:
    description: Build, test, and deploy to all environments
    steps:
      - workflow: build.yml
        wait_for: success      # Wait for successful completion (default)
        on_failure: abort      # Stop chain on failure (default)
      - workflow: test.yml
        wait_for: completion   # Wait for any completion (success or failure)
        on_failure: continue   # Continue even if this step fails
      - workflow: deploy.yml
        wait_for: none         # Don't wait, dispatch immediately
        inputs:
          environment: production
          version: v1.0.0

  quick-test:
    description: Run tests with default settings
    steps:
      - workflow: test.yml
```

### Chain Options

| Option | Values | Default | Description |
|--------|--------|---------|-------------|
| `wait_for` | `success`, `completion`, `none` | `success` | When to proceed to next step |
| `on_failure` | `abort`, `skip`, `continue` | `abort` | What to do when step fails |
| `inputs` | map | - | Override workflow inputs |

### Accessing Chains

1. Press `Tab` to focus the right panel
2. Press `l` to switch to the Chains tab
3. Navigate with `j`/`k` and press `Enter` to execute

The status bar shows `Chains(N)` when chains are configured, and `Chain: name (step/total)` during execution.

## Recording the Demo

Generate the demo GIFs using VHS:

```bash
# Main demo
vhs < .github/assets/demo.tape

# Chains demo
vhs < .github/assets/chains-demo.tape
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
