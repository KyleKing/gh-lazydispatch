// Command gh-lazydispatch is a TUI gh extension for dispatching and watching GitHub Actions workflows.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/theme"

	"github.com/kyleking/gh-lazydispatch/internal/app"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		showVersion bool
		showHelp    bool
	)

	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	flag.BoolVar(&showHelp, "help", false, "Show help")
	flag.BoolVar(&showHelp, "h", false, "Show help (shorthand)")
	flag.Parse()

	if showVersion {
		fmt.Printf("gh-lazydispatch %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	if showHelp {
		printHelp()
		os.Exit(0)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	workflows, failures, err := workflow.Discover(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering workflows: %v\n", err)
		os.Exit(1)
	}

	for _, failure := range failures {
		fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", failure.Filename, failure.Err)
	}

	if len(workflows) == 0 {
		fmt.Println("No dispatchable workflows found in .github/workflows/")
		fmt.Println("\nWorkflows must have 'workflow_dispatch' trigger to be dispatchable.")

		if len(failures) > 0 {
			fmt.Printf("\n%d workflow file(s) failed to parse, listed above.\n", len(failures))
		}

		os.Exit(0)
	}

	repo, err := runner.DetectRepo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not detect repository: %v\n", err)

		repo = "unknown/unknown"
	}

	history, err := frecency.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load history: %v\n", err)

		history = frecency.NewStore()
	}

	ui.InitTheme(theme.Detect())

	model := app.New(workflows, history, repo)

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`gh-lazydispatch - Interactive GitHub Workflow Dispatcher

Usage:
  gh-lazydispatch [flags]

Description:
  A TUI for triggering GitHub Actions workflow_dispatch workflows with
  fuzzy selection, interactive input configuration, and frecency-based
  history tracking.

Flags:
  -h, --help     Show this help message
  -v, --version  Show version (includes commit and build date)

Environment Variables:
  CATPPUCCIN_THEME   Override theme (latte/macchiato)

Keyboard Shortcuts:
  Tab / Shift+Tab    Switch between panes
  ↑/k, ↓/j           Navigate within pane
  h/←, l/→           Previous / next tab (right pane)
  Enter              Select / Execute workflow
  b                  Select branch
  w                  Toggle watch mode
  1-9                Select workflow, or edit input, by number
  C                  Run a chain
  L                  Live run overview
  v                  View logs for the selected entry
  ?                  Show help
  q, Ctrl+C          Quit

For more information: https://github.com/kyleking/gh-lazydispatch`)
}
