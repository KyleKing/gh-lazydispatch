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
	// Annotations are the failed jobs' check-run annotations, the only record
	// of a runner that died before it wrote a step log.
	Annotations []jobAnnotation `json:"annotations,omitempty"`
	Signatures  []signature     `json:"signatures"`
	RunID       int64           `json:"run_id"`
}

type jobAnnotation struct {
	Job     string `json:"job"`
	Level   string `json:"level"`
	Title   string `json:"title,omitempty"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type failedStep struct {
	StepName string `json:"step_name"`
	Job      string `json:"job,omitempty"`
	// LogNote says why the step has no log to read, so an empty tail is never
	// mistaken for a step that logged nothing wrong.
	LogNote    string `json:"log_note,omitempty"`
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
	result.Annotations = fetchAnnotations(client, runLogs, stderr)
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
		URL:         run.URL,
		FailedSteps: failedSteps(runLogs, tailLines, run.Conclusion != github.ConclusionSuccess),
		Signatures:  signatures(runLogs),
	}

	return result
}

// fetchAnnotations reads the annotations of every failed job. A job whose
// annotations cannot be read is named on stderr rather than dropped silently.
func fetchAnnotations(client *github.Client, runLogs *logs.RunLogs, stderr io.Writer) []jobAnnotation {
	var found []jobAnnotation

	seen := make(map[int64]bool)

	for _, step := range runLogs.AllSteps() {
		if step == nil || step.JobID == 0 || seen[step.JobID] || !step.JobFailed() {
			continue
		}

		seen[step.JobID] = true

		annotations, err := client.GetJobAnnotations(step.JobID)
		if err != nil {
			notef(stderr, "annotations for job %q unavailable: %v\n", step.JobName, err)

			continue
		}

		for _, annotation := range annotations {
			found = append(found, jobAnnotation{
				Job:     step.JobName,
				Level:   annotation.Level,
				Title:   annotation.Title,
				Message: annotation.Message,
				Path:    annotation.Path,
			})
		}
	}

	return found
}

// failedSteps keeps the tail of the first step that did not succeed in each
// failed job (a blank conclusion counts), because a later post step often fails
// only as a consequence. Outside a failed job it keeps every failed step. A run that
// failed while every step reports success still yields its last step, because a
// diagnosis with nothing in it is worse than one line too many. A run that
// succeeded yields none, so a step is never reported as failed on its own
// success.
func failedSteps(runLogs *logs.RunLogs, tailLines int, runFailed bool) []failedStep {
	all := runLogs.AllSteps()
	steps := make([]failedStep, 0, len(all))
	reportedJobs := make(map[int64]bool)

	for _, step := range all {
		if step == nil {
			continue
		}

		if step.JobID != 0 && step.JobFailed() {
			if reportedJobs[step.JobID] || !step.Unsuccessful() {
				continue
			}

			reportedJobs[step.JobID] = true
		} else if !step.Failed() {
			continue
		}

		steps = append(steps, tailOf(step, tailLines))
	}

	if runFailed && len(steps) == 0 && len(all) > 0 {
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
		Job:        step.JobName,
		LogNote:    logNote(step),
		Workflow:   step.Workflow,
		Conclusion: step.Conclusion,
		Errors:     errorLines,
		Tail:       lines[start:end],
		Omitted:    len(lines) - (end - start),
	}
}

func logNote(step *logs.StepLogs) string {
	switch {
	case step.Error != nil:
		return fmt.Sprintf("log unavailable: %v", step.Error)
	case len(step.Entries) == 0:
		return "no log: GitHub served no lines for this step (missing or empty)"
	default:
		return ""
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
	for i := range result.FailedSteps {
		total += len(result.FailedSteps[i].Tail) + len(result.FailedSteps[i].Errors)
	}

	return total
}
