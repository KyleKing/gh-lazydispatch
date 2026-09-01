package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

const exportUsage = `gh-lazydispatch export - read GitHub Actions without reading its logs

Usage:
  gh-lazydispatch export workflows           Dispatchable workflows and their inputs
  gh-lazydispatch export chains              Chains defined in .github/lazydispatch.yml
  gh-lazydispatch export runs [flags]        Recent workflow runs, newest first
  gh-lazydispatch export logs <run-id>       One run's logs, parsed into steps
  gh-lazydispatch export diagnose <run-id>   Why a run failed, without its logs

Flags (runs):
  --workflow <file>   Filename, e.g. ci.yml
  --branch <name>
  --status <status>   queued|in_progress|completed|success|failure
  --limit <n>         Default 20

Flags (logs):
  --errors-only       Keep only lines that read as errors
  --step <n>          One step, by index
  --grep <pattern>    Keep lines matching a regular expression
  --limit <n>         Cap lines per step
  --format json|md    Default json

Every command writes JSON to stdout and counts to stderr. Nothing here
dispatches: these commands only read.`

// runExport dispatches one export subcommand.
func runExport(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return ErrUsage
	}

	switch args[0] {
	case "workflows":
		return exportWorkflows(args[1:], stdout, stderr)
	case "chains":
		return exportChains(args[1:], stdout, stderr)
	case "runs":
		return exportRuns(args[1:], stdout, stderr)
	case "logs":
		return exportLogs(args[1:], stdout, stderr)
	case "diagnose":
		return exportDiagnose(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("%w: unknown export command %q", ErrUsage, args[0])
	}
}

// newFlagSet builds a flag set that reports usage through ErrUsage rather than
// printing its own and exiting.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	return fs
}

// workflowSummary is a dispatchable workflow reduced to what a caller needs to
// dispatch it: the filename to name it by, and every input with its type,
// default, and permitted values.
type workflowSummary struct {
	Inputs   map[string]inputSummary `json:"inputs"`
	Name     string                  `json:"name"`
	Filename string                  `json:"filename"`
}

type inputSummary struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required"`
}

func exportWorkflows(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("workflows")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving the working directory: %w", err)
	}

	files, failures, err := workflow.Discover(root)
	if err != nil {
		return fmt.Errorf("discovering workflows: %w", err)
	}

	for _, failure := range failures {
		notef(stderr, "skipped %s: %v\n", failure.Filename, failure.Err)
	}

	summaries := make([]workflowSummary, 0, len(files))
	for _, file := range files {
		summaries = append(summaries, summarizeWorkflow(file))
	}

	notef(stderr, "%d dispatchable workflows\n", len(summaries))

	return writeJSON(stdout, summaries)
}

func summarizeWorkflow(file workflow.File) workflowSummary {
	inputs := make(map[string]inputSummary)
	for name, input := range file.GetInputs() {
		inputs[name] = inputSummary{
			Type:        input.InputType(),
			Description: input.Description,
			Default:     input.Default,
			Options:     input.Options,
			Required:    input.Required,
		}
	}

	return workflowSummary{Name: file.Name, Filename: file.Filename, Inputs: inputs}
}

func exportChains(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("chains")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving the working directory: %w", err)
	}

	cfg, err := config.Load(root)
	if err != nil {
		return fmt.Errorf("loading %s: %w", config.ConfigFilename, err)
	}

	notef(stderr, "%d chains\n", len(cfg.Chains))

	return writeJSON(stdout, summarizeChains(cfg))
}

// chainSummary is a chain reduced to what a caller needs to run it: the
// variables it asks for and the steps it dispatches, in order.
type chainSummary struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Variables   []variableSummary `json:"variables,omitempty"`
	Steps       []stepSummary     `json:"steps"`
}

type variableSummary struct {
	Name        string   `json:"name"`
	Type        string   `json:"type,omitempty"`
	Description string   `json:"description,omitempty"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required"`
}

type stepSummary struct {
	Inputs    map[string]string `json:"inputs,omitempty"`
	Workflow  string            `json:"workflow"`
	WaitFor   string            `json:"wait_for,omitempty"`
	OnFailure string            `json:"on_failure,omitempty"`
}

func summarizeChains(cfg *config.WfdConfig) []chainSummary {
	names := cfg.ChainNames()
	summaries := make([]chainSummary, 0, len(names))

	for _, name := range names {
		chain := cfg.Chains[name]
		summary := chainSummary{Name: name, Description: chain.Description}

		for _, variable := range chain.Variables {
			summary.Variables = append(summary.Variables, variableSummary{
				Name:        variable.Name,
				Type:        variable.Type,
				Description: variable.Description,
				Default:     variable.Default,
				Options:     variable.Options,
				Required:    variable.Required,
			})
		}

		for _, step := range chain.Steps {
			summary.Steps = append(summary.Steps, stepSummary{
				Workflow:  step.Workflow,
				WaitFor:   string(step.WaitFor),
				OnFailure: string(step.OnFailure),
				Inputs:    step.Inputs,
			})
		}

		summaries = append(summaries, summary)
	}

	return summaries
}

// runSummary is a workflow run reduced to the fields that decide what to do
// next, so listing a hundred runs stays a few kilobytes.
type runSummary struct {
	CreatedAt  string `json:"created_at"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion,omitempty"`
	Branch     string `json:"branch"`
	URL        string `json:"url"`
	ID         int64  `json:"id"`
}

const defaultRunLimit = 20

func exportRuns(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("runs")
	query := github.RunQuery{}
	fs.StringVar(&query.Workflow, "workflow", "", "workflow filename")
	fs.StringVar(&query.Branch, "branch", "", "branch name")
	fs.StringVar(&query.Status, "status", "", "run status or conclusion")
	fs.IntVar(&query.Limit, "limit", defaultRunLimit, "maximum runs")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	runs, err := client.ListRuns(query)
	if err != nil {
		return fmt.Errorf("listing runs: %w", err)
	}

	summaries := make([]runSummary, 0, len(runs))
	for i := range runs {
		run := &runs[i]
		summaries = append(summaries, runSummary{
			ID:         run.ID,
			Name:       run.Name,
			Status:     run.Status,
			Conclusion: run.Conclusion,
			Branch:     run.HeadBranch,
			URL:        run.HTMLURL,
			CreatedAt:  run.CreatedAt.Format(timeFormat),
		})
	}

	notef(stderr, "%d runs\n", len(summaries))

	return writeJSON(stdout, summaries)
}

// newClient builds a GitHub client for the repository the working directory
// belongs to.
func newClient() (*github.Client, error) {
	repo, err := runner.DetectRepo()
	if err != nil {
		return nil, fmt.Errorf("detecting the repository: %w", err)
	}

	client, err := github.NewClient(repo)
	if err != nil {
		return nil, fmt.Errorf("building a GitHub client for %s: %w", repo, err)
	}

	return client, nil
}

// parseRunID pulls the run ID out of args wherever it sits and parses the
// rest as flags. Go's flag package stops at the first positional, so leaving
// the ID in place would silently discard every flag written after it.
func parseRunID(fs *flag.FlagSet, args []string) (int64, error) {
	rest := make([]string, 0, len(args))
	found := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if found != "" || strings.HasPrefix(arg, "-") {
			rest = append(rest, arg)

			if takesValue(fs, arg) && i+1 < len(args) {
				i++
				rest = append(rest, args[i])
			}

			continue
		}

		found = arg
	}

	if found == "" {
		return 0, fmt.Errorf("%w: expected a run ID", ErrUsage)
	}

	if err := fs.Parse(rest); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrUsage, err)
	}

	if fs.NArg() > 0 {
		return 0, fmt.Errorf("%w: unexpected argument %q", ErrUsage, fs.Arg(0))
	}

	runID, err := strconv.ParseInt(found, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q is not a run ID", ErrUsage, found)
	}

	return runID, nil
}

// takesValue reports whether name is a declared flag written as "--flag value"
// rather than "--flag=value", so its value is not mistaken for the run ID.
func takesValue(fs *flag.FlagSet, name string) bool {
	trimmed := strings.TrimLeft(name, "-")
	if strings.Contains(trimmed, "=") {
		return false
	}

	found := fs.Lookup(trimmed)
	if found == nil {
		return false
	}

	boolFlag, ok := found.Value.(interface{ IsBoolFlag() bool })

	return !ok || !boolFlag.IsBoolFlag()
}

// fetchRunLogs pulls and parses one run's logs through the same fetcher the
// TUI uses.
func fetchRunLogs(client *github.Client, runID int64) (*logs.RunLogs, error) {
	steps, err := logs.NewGHFetcher(client).FetchStepLogsReal(runID, "")
	if err != nil {
		return nil, fmt.Errorf("fetching logs for run %d: %w", runID, err)
	}

	runLogs := logs.NewRunLogs(fmt.Sprintf("run-%d", runID), "")
	for _, step := range steps {
		runLogs.AddStep(step)
	}

	return runLogs, nil
}
