# gh-wfr

![.github/assets/demo.gif](https://raw.githubusercontent.com/kyleking/gh-wfr/main/.github/assets/demo.gif)

Interactive GitHub Actions workflow dispatcher TUI with fuzzy selection, input configuration, and frecency-based history.

## Features

- Fuzzy search for workflow selection
- Interactive input configuration for workflow_dispatch inputs
- Branch selection
- Watch mode for real-time workflow run updates
- Frecency-based workflow history tracking
- Theme support (Catppuccin)
- Command preview before execution

## Installation

### As a GitHub CLI Extension (Recommended)

```bash
gh extension install KyleKing/gh-wfr
```

Then run with:

```bash
gh wfr
```

### Standalone Binary

```bash
go install github.com/kyleking/gh-wfr@latest
```

Or build from source:

```bash
git clone https://github.com/kyleking/gh-wfr
cd gh-wfr
go build
```

## Usage

Navigate to a directory with a Git repository containing GitHub Actions workflows:

```bash
cd your-project

# If installed as gh extension:
gh wfr

# If installed as standalone:
gh-wfr
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
