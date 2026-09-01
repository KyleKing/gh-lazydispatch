# gh-lazydispatch docs

A Bubble Tea TUI that discovers the `workflow_dispatch` workflows in the current repository, configures their inputs, dispatches them, and streams back run status and logs. Chains run several workflows in sequence with wait conditions and failure handling.

## Pages

- [Interface](./interface.md) for the panes, the keys, and the log viewer
- [Export commands](./cli.md) for reading runs, logs, and failures without the TUI
- [Chains](./chains.md) for running several workflows in sequence
- [Chain examples](./chain-examples.md) for worked chain configurations
- [Configuration](./configuration.md) for the config file and environment variables
- [Troubleshooting](./troubleshooting.md) for auth failures and empty workflow lists
- [Alternatives](./alternatives.md) for the tools that cover what this leaves out
- [Development](./development.md) for the dependency-update routine
- [Live dispatch testing](./live-testing.md) for exercising the real dispatch path
- [UX mockups](../UX.md) for layout sketches

Setup, tasks, and the release flow live in [CONTRIBUTING.md](../CONTRIBUTING.md). Architecture lives in [DESIGN.md](../DESIGN.md), and planned work in [ROADMAP.md](../ROADMAP.md).

## Requirements

- A git repository with GitHub Actions workflows, as the working directory
- gh CLI, logged in. Dispatch and log viewing both go through it, so run `gh auth login` first
- Go 1.25+, only to build or `go install` from source

## What it gives you

- Fuzzy search over the workflows that accept `workflow_dispatch`
- Interactive input configuration, typed from each workflow's declared inputs
- Branch selection sorted by frecency
- Watch mode for live run updates
- Frecency-based history of what you have dispatched
- Workflow chains for multi-step deployments
- Log viewer with per-step tabs, filtering, search, and live streaming
- A tabbed right panel holding History, Chains, Live, Timeline, and Runs, with each tab's count in the tab bar
- A command preview before anything runs
- Catppuccin Latte or Macchiato
