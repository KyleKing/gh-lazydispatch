package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

// stepExport is one step's parsed lines. Timestamps and escape sequences are
// already stripped by the fetcher, so this is the log without the transport.
type stepExport struct {
	Workflow   string   `json:"workflow"`
	StepName   string   `json:"step_name"`
	Status     string   `json:"status,omitempty"`
	Conclusion string   `json:"conclusion,omitempty"`
	Lines      []string `json:"lines"`
	StepIndex  int      `json:"step_index"`
	Truncated  int      `json:"truncated,omitempty"`
}

type logsExport struct {
	Steps []stepExport `json:"steps"`
	RunID int64        `json:"run_id"`
}

func exportLogs(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("logs")
	cfg := logs.NewFilterConfig()

	var (
		errorsOnly bool
		format     string
		limit      int
	)

	fs.BoolVar(&errorsOnly, "errors-only", false, "keep only error lines")
	fs.StringVar(&cfg.SearchTerm, "grep", "", "keep lines matching a regular expression")
	fs.IntVar(&cfg.StepIndex, "step", -1, "one step, by index")
	fs.IntVar(&limit, "limit", 0, "cap lines per step")
	fs.StringVar(&format, "format", "json", "json or md")

	runID, err := parseRunID(fs, args)
	if err != nil {
		return err
	}

	if errorsOnly {
		cfg.Level = logs.FilterErrors
	}

	cfg.Regex = cfg.SearchTerm != ""

	client, err := newClient()
	if err != nil {
		return err
	}

	runLogs, err := fetchRunLogs(client, runID)
	if err != nil {
		return err
	}

	return writeLogs(stdout, stderr, runLogs, cfg, format, runID, limit)
}

func writeLogs(
	stdout, stderr io.Writer, runLogs *logs.RunLogs, cfg *logs.FilterConfig,
	format string, runID int64, limit int,
) error {
	read := countLines(runLogs)

	if format == "md" {
		doc, err := logs.ExportAsMarkdown(runLogs, cfg)
		if err != nil {
			return fmt.Errorf("rendering markdown: %w", err)
		}

		reportKept(stderr, read, strings.Count(doc, "\n"))
		_, err = io.WriteString(stdout, doc)

		return err //nolint:wrapcheck // the writer's error is the caller's own
	}

	if format != "json" {
		return fmt.Errorf("%w: unknown format %q", ErrUsage, format)
	}

	filter, err := logs.NewFilter(cfg)
	if err != nil {
		return fmt.Errorf("building the filter: %w", err)
	}

	export := logsExport{RunID: runID, Steps: collectSteps(runLogs, filter.Apply(runLogs), limit)}

	kept := 0
	for _, step := range export.Steps {
		kept += len(step.Lines)
	}

	reportKept(stderr, read, kept)

	return writeJSON(stdout, export)
}

// collectSteps pairs each filtered step with the metadata the filter drops,
// and applies the per-step line cap.
func collectSteps(runLogs *logs.RunLogs, result *logs.FilteredResult, limit int) []stepExport {
	meta := make(map[int]*logs.StepLogs)
	for _, step := range runLogs.AllSteps() {
		meta[step.StepIndex] = step
	}

	steps := make([]stepExport, 0, len(result.Steps))

	for _, filtered := range result.Steps {
		lines := make([]string, 0, len(filtered.Entries))
		for _, entry := range filtered.Entries {
			lines = append(lines, entry.Original.Content)
		}

		truncated := 0
		if limit > 0 && len(lines) > limit {
			truncated = len(lines) - limit
			lines = lines[:limit]
		}

		step := stepExport{
			StepIndex: filtered.StepIndex,
			Workflow:  filtered.Workflow,
			StepName:  filtered.StepName,
			Lines:     lines,
			Truncated: truncated,
		}

		if original, ok := meta[filtered.StepIndex]; ok {
			step.Status = original.Status
			step.Conclusion = original.Conclusion
		}

		steps = append(steps, step)
	}

	return steps
}

func countLines(runLogs *logs.RunLogs) int {
	total := 0
	for _, step := range runLogs.AllSteps() {
		total += len(step.Entries)
	}

	return total
}

// reportKept states on stderr how much of the run was read and how much
// reached stdout, so a caller can see what the filter saved rather than trust
// that it saved anything.
func reportKept(stderr io.Writer, read, kept int) {
	notef(stderr, "read %d lines, emitted %d\n", read, kept)
}
