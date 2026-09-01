package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/ghcassette"
)

func TestMain(m *testing.M) {
	code := m.Run()

	ghcassette.RemoveStub()
	os.Exit(code)
}

// The runs the cassettes were recorded against, both completed and both read
// rather than dispatched. They differ in the only way that matters here:
// whether `gh run view --log` could name the steps its lines belong to.
const (
	recordedRepo = "KyleKing/gh-lazydispatch"

	// RecordedStepRun is a Demo Chain Check run whose log names each step, and
	// whose failing step echoes a case statement mentioning every signature
	// Detect looks for while emitting only one of them.
	recordedStepRun = "33467068056"

	// RecordedUnknownStepRun is a CI run whose every log line reads
	// "UNKNOWN STEP", which is what gh writes when it cannot map a line to a
	// declared step. Parsing it strictly yields nothing at all.
	recordedUnknownStepRun = "33423560774"
)

// runRecorded replays the named cassette through the real command path, so the
// bytes under test are the ones GitHub sent.
func runRecorded(t *testing.T, name string, args ...string) (string, string, int) {
	t.Helper()

	path, err := filepath.Abs(filepath.Join("testdata", "cassettes", name+".golden"))
	if err != nil {
		t.Fatalf("resolving the cassette path: %v", err)
	}

	session := ghcassette.Start(t, path)
	session.Apply(t)
	t.Setenv("GH_REPO", recordedRepo)

	var stdout, stderr bytes.Buffer

	code := Run(append([]string{"export"}, args...), &stdout, &stderr)

	session.RequireAllPlayed(t)

	return stdout.String(), stderr.String(), code
}

// TestRecorded_DiagnoseReadsAJobGHCouldNotMapToSteps is the regression test for
// a run whose log carries no step names. Grouping strictly on declared step
// names dropped every line of it, so diagnose reported a failed run with
// nothing wrong.
//
//nolint:paralleltest // runRecorded calls t.Setenv, which rules out t.Parallel
func TestRecorded_DiagnoseReadsAJobGHCouldNotMapToSteps(t *testing.T) {
	stdout, stderr, code := runRecorded(t, "unknown-step-run", "diagnose", recordedUnknownStepRun)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	var result diagnosis
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, stdout)
	}

	if len(result.FailedSteps) == 0 {
		t.Fatal("no failed steps, so every log line was dropped")
	}

	if !strings.HasPrefix(stderr, "read 4088 lines") {
		t.Errorf("stderr reports %q, want the full line count it read", strings.TrimSpace(stderr))
	}

	step := result.FailedSteps[0]
	if len(step.Errors) == 0 {
		t.Fatal("the failed step reports no error lines")
	}

	// The tail must end on the failure rather than on the job's teardown,
	// which is what a job-sized step's last lines otherwise are.
	last := step.Tail[len(step.Tail)-1]
	if !strings.Contains(last, "##[error]") {
		t.Errorf("the tail ends on %q, want the line the step failed on", last)
	}
}

// TestRecorded_DiagnoseIgnoresTheEchoedScript holds diagnose to reporting what
// a run did rather than what its script says. GitHub folds each `run:` block
// into the log, so a case statement naming six failure modes is not six
// failures.
//
//nolint:paralleltest // runRecorded calls t.Setenv, which rules out t.Parallel
func TestRecorded_DiagnoseIgnoresTheEchoedScript(t *testing.T) {
	stdout, _, code := runRecorded(t, "step-run", "diagnose", recordedStepRun)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	var result diagnosis
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}

	labels := make([]string, 0, len(result.Signatures))
	for _, found := range result.Signatures {
		labels = append(labels, found.Label)
	}

	if len(labels) != 1 || labels[0] != "Network failure" {
		t.Errorf("signatures are %v, want only the one the run emitted", labels)
	}
}

// TestRecorded_DiagnoseCostsLessThanTheLogItRead is the claim the command
// exists for, measured rather than asserted.
//
//nolint:paralleltest // runRecorded calls t.Setenv, which rules out t.Parallel
func TestRecorded_DiagnoseCostsLessThanTheLogItRead(t *testing.T) {
	stdout, _, code := runRecorded(t, "unknown-step-run", "diagnose", recordedUnknownStepRun)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	raw, err := os.ReadFile(filepath.Join("testdata", "cassettes", "unknown-step-run.golden"))
	if err != nil {
		t.Fatalf("reading the cassette: %v", err)
	}

	const wantRatio = 20
	if ratio := len(raw) / len(stdout); ratio < wantRatio {
		t.Errorf("diagnose emitted %d bytes from a %d byte log (%dx), want at least %dx",
			len(stdout), len(raw), ratio, wantRatio)
	}
}

// TestRecorded_LogsRenderMarkdownForPasting covers the other output shape:
// a document a reader drops into an issue, with the signatures at the top and
// no terminal escapes left in the body.
//
//nolint:paralleltest // runRecorded calls t.Setenv, which rules out t.Parallel
func TestRecorded_LogsRenderMarkdownForPasting(t *testing.T) {
	stdout, stderr, code := runRecorded(t, "logs-markdown", "logs", recordedStepRun, "--format", "md")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	for _, want := range []string{"# run-", "## Detected issues", "```"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the document is missing %q", want)
		}
	}

	if strings.Contains(stdout, "^[[") {
		t.Error("the document carries terminal escapes, which a pasted issue renders verbatim")
	}
}

//nolint:paralleltest // runRecorded calls t.Setenv, which rules out t.Parallel
func TestRecorded_LogsRejectAnUnknownFormat(t *testing.T) {
	_, stderr, code := runRecorded(t, "logs-markdown", "logs", recordedStepRun, "--format", "yaml")
	if code != 2 {
		t.Fatalf("exit %d, want 2 for an unknown format", code)
	}

	if !strings.Contains(stderr, "yaml") {
		t.Errorf("stderr does not name the format it rejected: %q", stderr)
	}
}

// TestRecorded_RunsListsWhatRanRecently keeps the listing to the fields that
// decide what to do next, so asking about a hundred runs stays cheap.
//
//nolint:paralleltest // runRecorded calls t.Setenv, which rules out t.Parallel
func TestRecorded_RunsListsWhatRanRecently(t *testing.T) {
	stdout, stderr, code := runRecorded(t, "runs", "runs", "--workflow", "ci.yml", "--limit", "5")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	var found []runSummary
	if err := json.Unmarshal([]byte(stdout), &found); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}

	if len(found) == 0 {
		t.Fatal("no runs came back")
	}

	if len(found) > 5 {
		t.Errorf("--limit 5 returned %d runs", len(found))
	}

	for _, run := range found {
		if run.ID == 0 || run.Status == "" || run.URL == "" {
			t.Errorf("run %+v is missing a field a caller needs", run)
		}
	}
}

// TestRecorded_LogsErrorsOnlySkipsTheEchoedScript is the JSON counterpart to
// the markdown document, and the level filter's own regression test: a step's
// script says "Error:" in every branch it never took.
//
//nolint:paralleltest // runRecorded calls t.Setenv, which rules out t.Parallel
func TestRecorded_LogsErrorsOnlySkipsTheEchoedScript(t *testing.T) {
	stdout, stderr, code := runRecorded(t, "logs-markdown", "logs", recordedStepRun, "--errors-only")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	var export logsExport
	if err := json.Unmarshal([]byte(stdout), &export); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}

	if export.RunID == 0 || len(export.Steps) == 0 {
		t.Fatalf("nothing came back: %+v", export)
	}

	kept := 0

	for _, step := range export.Steps {
		if step.StepName == "" {
			t.Errorf("step %d came back with no name", step.StepIndex)
		}

		for _, line := range step.Lines {
			kept++

			if strings.HasPrefix(line, "##[group]Run ") || strings.HasPrefix(line, "  echo ") {
				t.Errorf("--errors-only kept a line from the echoed script: %q", line)
			}
		}
	}

	if !strings.Contains(stderr, "emitted "+strconv.Itoa(kept)) {
		t.Errorf("stderr reports %q, which does not match the %d lines emitted", strings.TrimSpace(stderr), kept)
	}
}
