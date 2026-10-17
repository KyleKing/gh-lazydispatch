package logs

import (
	"os"
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// The fixture is real `gh run view --log` output. Its lines carry a
// "job\tstep\ttimestamp message" prefix, so a parser looking for ##[group] at
// the start of a line finds no steps at all and the viewer reads as empty.
func TestParseJobLogsIntoSteps(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/gh_run_view.log")
	if err != nil {
		t.Fatal(err)
	}

	job := github.Job{
		Name: "test",
		Steps: []github.Step{
			{Name: "Set up job", Status: "completed", Conclusion: "success"},
			{Name: "Echo inputs", Status: "completed", Conclusion: "success"},
		},
	}

	steps := (&GHFetcher{}).parseJobLogsIntoSteps(&job, string(raw), "demo-test.yml", 42, 0)
	if len(steps) == 0 {
		t.Fatal("no steps parsed from real gh output")
	}

	first := steps[0]
	if first.StepName != "Set up job" {
		t.Errorf("first step is %q, want %q", first.StepName, "Set up job")
	}

	if first.Conclusion != "success" {
		t.Errorf("step conclusion is %q, want it taken from the job metadata", first.Conclusion)
	}

	if len(first.Entries) == 0 {
		t.Error("first step carries no log entries")
	}

	for _, entry := range first.Entries {
		if entry.StepName != first.StepName {
			t.Errorf("entry attributed to %q, want %q", entry.StepName, first.StepName)
		}

		if entry.Content != "" && entry.Content[0] == '\t' {
			t.Errorf("entry still carries the tab-separated prefix: %q", entry.Content)
		}
	}
}

// A runner that dies inside a step leaves that step with a blank conclusion and
// no log. Dropping it leaves the step that failed as the one a diagnosis cannot
// see.
func TestParseJobLogsIntoSteps_KeepsAnUnsuccessfulStepThatHasNoLog(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/gh_run_view.log")
	if err != nil {
		t.Fatal(err)
	}

	job := github.Job{
		ID:         9,
		Name:       "test",
		Status:     github.StatusCompleted,
		Conclusion: github.ConclusionFailure,
		Steps: []github.Step{
			{Name: "Set up job", Status: "completed", Conclusion: "success"},
			{Name: "Echo inputs", Status: "completed", Conclusion: "success"},
			{Name: "Pulumi up (prod)", Status: "completed"},
			{Name: "Skipped step", Status: "completed", Conclusion: "skipped"},
		},
	}

	steps := (&GHFetcher{}).parseJobLogsIntoSteps(&job, string(raw), "demo-test.yml", 42, 0)

	last := steps[len(steps)-1]
	if last.StepName != "Pulumi up (prod)" || len(last.Entries) != 0 {
		t.Fatalf("last step = %q with %d entries, want the logless step", last.StepName, len(last.Entries))
	}

	if !last.Unsuccessful() || last.JobID != 9 || !last.JobFailed() {
		t.Errorf("step = %+v, want it unsuccessful and tied to its failed job", last)
	}

	for _, step := range steps {
		if step.StepName == "Skipped step" {
			t.Error("a skipped step was reported")
		}
	}
}

// GitHub stores its own coloring as caret notation, not as escape bytes, so
// ansi.Strip alone leaves it in the line.
func TestParseLogOutput_StripsBothEscapeSpellings(t *testing.T) {
	t.Parallel()

	raw := "2026-09-01T03:18:13.0741276Z ^[[36;1m    echo \"Error: boom\"^[[0m\n" +
		"2026-09-01T03:18:14.0000000Z \x1b[31mreal escape\x1b[0m"

	entries := ParseLogOutput(raw, "step")
	if len(entries) != 2 {
		t.Fatalf("parsed %d entries, want 2", len(entries))
	}

	if got := entries[0].Content; got != `    echo "Error: boom"` {
		t.Errorf("caret-notation escapes survived: %q", got)
	}

	if got := entries[1].Content; got != "real escape" {
		t.Errorf("escape bytes survived: %q", got)
	}

	if entries[0].Level != LogLevelError {
		t.Errorf("level is %q, want error", entries[0].Level)
	}
}
