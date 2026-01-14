package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-wfd/internal/app"
	"github.com/kyleking/gh-wfd/internal/frecency"
	"github.com/kyleking/gh-wfd/internal/runner"
	"github.com/kyleking/gh-wfd/internal/ui"
	"github.com/kyleking/gh-wfd/internal/ui/theme"
	"github.com/kyleking/gh-wfd/internal/workflow"
)

var (
	version = "dev"
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
		fmt.Printf("gh-wfd %s\n", version)
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

	workflows, err := workflow.Discover(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering workflows: %v\n", err)
		os.Exit(1)
	}

	if len(workflows) == 0 {
		fmt.Println("No dispatchable workflows found in .github/workflows/")
		fmt.Println("\nWorkflows must have 'workflow_dispatch' trigger to be dispatchable.")
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

	detectedTheme := theme.Detect()
	ui.InitTheme(detectedTheme)

	model := app.New(workflows, history, repo)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`gh-wfd - Interactive GitHub Workflow Dispatcher

Usage:
  gh wfd [flags]

Description:
  A TUI for triggering GitHub Actions workflow_dispatch workflows with
  fuzzy selection, interactive input configuration, and frecency-based
  history tracking.

Flags:
  -h, --help     Show this help message
  -v, --version  Show version

Environment Variables:
  CATPPUCCIN_THEME   Override theme (latte/macchiato)

Keyboard Shortcuts:
  Tab / Shift+Tab    Switch between panes
  ↑/k, ↓/j           Navigate within pane
  Enter              Select / Execute workflow
  b                  Select branch
  w                  Toggle watch mode
  1-9                Edit input by number
  ?                  Show help
  q, Ctrl+C          Quit

For more information: https://github.com/kyleking/gh-wfd`)
}
