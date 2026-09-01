package cli

import (
	"fmt"
	"io"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

// defaultTailLines is how much of a failed step diagnose keeps. A step's
// failure is almost always in its last lines, and the whole point of this
// command is to not return the rest.
const defaultTailLines = 20

// diagnosis is what a caller needs to decide what to do about a failed run:
// which steps failed, what the tail of each one says, and which known failure
// signatures matched.
type diagnosis struct {
	Name        string       `json:"name"`
	Status      string       `json:"status"`
	Conclusion  string       `json:"conclusion,omitempty"`
	Branch      string       `json:"branch,omitempty"`
	URL         string       `json:"url,omitempty"`
	FailedSteps []failedStep `json:"failed_steps"`
	Signatures  []signature  `json:"signatures"`
	RunID       int64        `json:"run_id"`
}

type failedStep struct {
	StepName   string `json:"step_name"`
	Workflow   string `json:"workflow,omitempty"`
	Conclusion string `json:"conclusion,omitempty"`
	// Errors holds every line the parser read as an error, which is what the
	// step was doing when it gave up.
	Errors []string `json:"errors,omitempty"`
	// Tail is the window ending at the last error, or at the step's end when
	// it logged none. A job's last lines are its teardown, so anchoring on the
	// error is what keeps the excerpt on the failure.
	Tail      []string `json:"tail"`
	StepIndex int      `json:"step_index"`
	Omitted   int      `json:"omitted,omitempty"`
}

// signature is one known failure pattern found in the run, with the line that
// matched and what to try first.
type signature struct {
	Label     string `json:"label"`
	Hint      string `json:"hint"`
	Workflow  string `json:"workflow,omitempty"`
	Line      string `json:"line"`
	StepIndex int    `json:"step_index"`
}

func exportDiagnose(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("diagnose")

	var tailLines int

	fs.IntVar(&tailLines, "tail", defaultTailLines, "lines to keep from each failed step; 0 for signatures only")

	runID, err := parseRunID(fs, args)
	if err != nil {
		return err
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	run, err := client.GetWorkflowRun(runID)
	if err != nil {
		return fmt.Errorf("fetching run %d: %w", runID, err)
	}

	runLogs, err := fetchRunLogs(client, runID)
	if err != nil {
		return err
	}

	result := buildDiagnosis(run, runLogs, tailLines)
	reportKept(stderr, countLines(runLogs), countTail(result))

	return writeJSON(stdout, result)
}

func buildDiagnosis(run *github.WorkflowRun, runLogs *logs.RunLogs, tailLines int) diagnosis {
	result := diagnosis{
		RunID:       run.ID,
		Name:        run.Name,
		Status:      run.Status,
		Conclusion:  run.Conclusion,
		Branch:      run.HeadBranch,
		URL:         run.HTMLURL,
		FailedSteps: failedSteps(runLogs, tailLines),
		Signatures:  signatures(runLogs),
	}

	return result
}

// failedSteps keeps the tail of every step that did not succeed. A run whose
// steps all report success but which failed anyway still yields its last step,
// because a diagnosis with nothing in it is worse than one line too many.
func failedSteps(runLogs *logs.RunLogs, tailLines int) []failedStep {
	all := runLogs.AllSteps()
	steps := make([]failedStep, 0, len(all))

	for _, step := range all {
		if step == nil || !step.Failed() {
			continue
		}

		steps = append(steps, tailOf(step, tailLines))
	}

	if len(steps) == 0 && len(all) > 0 {
		steps = append(steps, tailOf(all[len(all)-1], tailLines))
	}

	return steps
}

// maxErrorLines caps the error list, so a step that failed ten thousand times
// reports the shape of the failure rather than all of it.
const maxErrorLines = 20

func tailOf(step *logs.StepLogs, tailLines int) failedStep {
	if tailLines < 0 {
		tailLines = 0
	}

	lines := make([]string, 0, len(step.Entries))
	errorLines := make([]string, 0, maxErrorLines)
	end := len(step.Entries)

	var echo logs.CommandEcho

	for i, entry := range step.Entries {
		lines = append(lines, entry.Content)

		if entry.Level != logs.LogLevelError || echo.Echoed(entry.Content) {
			continue
		}

		end = i + 1

		// The cap keeps the last errors, not the first. A test runner narrating
		// 13,000 passing tests logs error-shaped progress lines for minutes
		// before the failure, so keeping the head reports the noise and drops
		// the summary that names the cause.
		if len(errorLines) == maxErrorLines {
			errorLines = errorLines[1:]
		}

		errorLines = append(errorLines, entry.Content)
	}

	start := max(end-tailLines, 0)

	return failedStep{
		StepIndex:  step.StepIndex,
		StepName:   step.StepName,
		Workflow:   step.Workflow,
		Conclusion: step.Conclusion,
		Errors:     errorLines,
		Tail:       lines[start:end],
		Omitted:    len(lines) - (end - start),
	}
}

func signatures(runLogs *logs.RunLogs) []signature {
	detections := logs.Detect(runLogs)
	found := make([]signature, 0, len(detections))

	for _, detection := range detections {
		found = append(found, signature{
			Label:     detection.Label,
			Hint:      detection.Hint,
			Workflow:  detection.Workflow,
			StepIndex: detection.StepIndex,
			Line:      detection.Line,
		})
	}

	return found
}

func countTail(result diagnosis) int {
	total := len(result.Signatures)
	for _, step := range result.FailedSteps {
		total += len(step.Tail) + len(step.Errors)
	}

	return total
}
