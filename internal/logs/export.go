package logs

import (
	"fmt"
	"strings"
)

// codeFence opens and closes a log body. Log lines carry no backticks of their
// own, so three is always enough.
const codeFence = "```"

// ExportAsMarkdown renders a run's logs as a markdown document: a heading per
// step with its status, the detected failure signatures, and each step's lines
// in a fenced block. A nil filter exports everything.
//
// The document is lossy by design: it holds what a reader pastes into an issue,
// not a byte-exact copy of the run's output.
func ExportAsMarkdown(runLogs *RunLogs, config *FilterConfig) (string, error) {
	if runLogs == nil {
		return "", nil
	}

	steps, err := exportSteps(runLogs, config)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	writeExportHeader(&sb, runLogs)
	writeDetections(&sb, Detect(runLogs))

	if len(steps) == 0 {
		sb.WriteString("\nNo log lines matched the active filter.\n")

		return sb.String(), nil
	}

	for _, step := range steps {
		writeExportStep(&sb, step)
	}

	return sb.String(), nil
}

// exportStep is one step reduced to what the document shows.
type exportStep struct {
	workflow   string
	stepName   string
	status     string
	conclusion string
	lines      []string
	index      int
}

func exportSteps(runLogs *RunLogs, config *FilterConfig) ([]exportStep, error) {
	all := runLogs.AllSteps()

	kept, err := keptEntries(runLogs, config)
	if err != nil {
		return nil, err
	}

	steps := make([]exportStep, 0, len(all))

	for _, step := range all {
		if step == nil {
			continue
		}

		lines, ok := kept[step.StepIndex]
		if !ok {
			continue
		}

		steps = append(steps, exportStep{
			index:      step.StepIndex,
			workflow:   step.Workflow,
			stepName:   step.StepName,
			status:     step.Status,
			conclusion: step.Conclusion,
			lines:      lines,
		})
	}

	return steps, nil
}

// keptEntries maps a step index to the lines that survived the filter. A step
// absent from the map contributed nothing and is left out of the document.
func keptEntries(runLogs *RunLogs, config *FilterConfig) (map[int][]string, error) {
	kept := make(map[int][]string)

	if config == nil {
		for _, step := range runLogs.AllSteps() {
			if step == nil {
				continue
			}

			lines := make([]string, 0, len(step.Entries))
			for _, entry := range step.Entries {
				lines = append(lines, entry.Content)
			}

			kept[step.StepIndex] = lines
		}

		return kept, nil
	}

	filter, err := NewFilter(config)
	if err != nil {
		return nil, err
	}

	for _, step := range filter.Apply(runLogs).Steps {
		lines := make([]string, 0, len(step.Entries))
		for _, entry := range step.Entries {
			lines = append(lines, entry.Original.Content)
		}

		kept[step.StepIndex] = lines
	}

	return kept, nil
}

func writeExportHeader(sb *strings.Builder, runLogs *RunLogs) {
	name := runLogs.ChainName
	if name == "" {
		name = "Workflow run"
	}

	fmt.Fprintf(sb, "# %s\n", name)

	if runLogs.Branch != "" {
		fmt.Fprintf(sb, "\nBranch: `%s`\n", runLogs.Branch)
	}
}

func writeDetections(sb *strings.Builder, detections []Detection) {
	if len(detections) == 0 {
		return
	}

	sb.WriteString("\n## Detected issues\n\n")

	for _, d := range detections {
		fmt.Fprintf(sb, "- **%s** in `%s`: %s\n", d.Label, d.workflowLabel(), d.Hint)
		fmt.Fprintf(sb, "  - `%s`\n", strings.TrimSpace(d.Line))
	}
}

// workflowLabel names the step a detection came from, falling back to its
// index when the run carries no workflow name.
func (d Detection) workflowLabel() string {
	if d.Workflow != "" {
		return d.Workflow
	}

	return fmt.Sprintf("step %d", d.StepIndex+1)
}

func writeExportStep(sb *strings.Builder, step exportStep) {
	title := step.workflow
	if title == "" {
		title = step.stepName
	}

	if title == "" {
		title = fmt.Sprintf("Step %d", step.index+1)
	}

	fmt.Fprintf(sb, "\n## %d. %s\n", step.index+1, title)

	if status := stepStatusLine(step); status != "" {
		fmt.Fprintf(sb, "\n%s\n", status)
	}

	sb.WriteString("\n" + codeFence + "\n")

	for _, line := range step.lines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString(codeFence + "\n")
}

func stepStatusLine(step exportStep) string {
	switch {
	case step.status != "" && step.conclusion != "":
		return fmt.Sprintf("Status: %s (%s)", step.status, step.conclusion)
	case step.status != "":
		return "Status: " + step.status
	case step.conclusion != "":
		return "Status: " + step.conclusion
	default:
		return ""
	}
}
