package logs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/ghcassette"

	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/github"
)

func TestMain(m *testing.M) {
	code := m.Run()

	ghcassette.RemoveStub()
	os.Exit(code)
}

// The repository and runs the cassettes were recorded against. Every one of
// them is a completed run of a dispatch-only demo workflow, so re-recording
// reads history rather than creating anything.
const (
	recordedRepo = "KyleKing/gh-lazydispatch"

	// RecordedPassingRun: Demo Test Suite, whose steps all succeeded.
	recordedPassingRun = 33467036043

	// RecordedFailingRun: Demo Chain Check, whose last step exits non-zero
	// after printing a line matching every signature Detect looks for.
	recordedFailingRun = 33467068056
)

func cassette(t *testing.T, name string) string {
	t.Helper()

	path, err := filepath.Abs(filepath.Join("testdata", "cassettes", name+".golden"))
	if err != nil {
		t.Fatalf("resolving the cassette path: %v", err)
	}

	return path
}

// recordedFetcher points this process's gh calls at the named cassette and
// returns a fetcher wired to the real client, so the bytes under test are the
// ones GitHub sent.
func recordedFetcher(t *testing.T, name string) (*GHFetcher, *ghcassette.Session) {
	t.Helper()

	s := ghcassette.Start(t, cassette(t, name))
	s.Apply(t)

	client, err := github.NewClientWithExecutor(recordedRepo, exec.NewRealExecutor())
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}

	return NewGHFetcher(client), s
}

// TestRecorded_StepLogsFromRealGHOutput is the test the hand-written fixtures
// could not be: gh prefixes every log line with "job\tstep\ttimestamp", and a
// parser written against ##[group] markers finds no steps at all. Fixtures
// authored in the shape their author imagined let that ship.
//
//nolint:paralleltest // Apply sets the process environment
func TestRecorded_StepLogsFromRealGHOutput(t *testing.T) {
	fetcher, s := recordedFetcher(t, "passing-run")

	steps, err := fetcher.FetchStepLogsReal(recordedPassingRun, "demo-test.yml")
	if err != nil {
		t.Fatalf("fetching step logs: %v", err)
	}

	if len(steps) == 0 {
		t.Fatal("no steps parsed from recorded gh output")
	}

	s.RequireAllPlayed(t)

	var total int

	for _, step := range steps {
		total += len(step.Entries)

		if step.StepName == "" {
			t.Errorf("step %d has no name, so the log prefix was not read", step.StepIndex)
		}

		for _, entry := range step.Entries {
			if strings.HasPrefix(entry.Content, step.JobName+"\t") {
				t.Errorf("entry still carries the tab-separated prefix: %q", entry.Content)
			}

			if strings.Contains(entry.Content, "\x1b[") || strings.Contains(entry.Content, "^[[") {
				t.Errorf("entry still carries terminal escapes: %q", entry.Content)
			}

			if entry.Timestamp.Year() != 2026 {
				t.Errorf("entry timestamp is %v, so it was stamped rather than parsed", entry.Timestamp)
			}
		}
	}

	if total == 0 {
		t.Error("steps parsed but carried no log lines")
	}
}

// TestRecorded_FailureSignaturesAndExport drives the whole pipeline the log
// viewer runs: fetch, filter to errors, detect the signatures, export markdown.
//
//nolint:paralleltest // Apply sets the process environment
func TestRecorded_FailureSignaturesAndExport(t *testing.T) {
	fetcher, s := recordedFetcher(t, "failing-run")

	steps, err := fetcher.FetchStepLogsReal(recordedFailingRun, "demo-chain-check.yml")
	if err != nil {
		t.Fatalf("fetching step logs: %v", err)
	}

	s.RequireAllPlayed(t)

	runLogs := NewRunLogs("demo-pipeline", "main")
	for _, step := range steps {
		runLogs.AddStep(step)
	}

	// The recorded workflow is a case statement naming every signature, and
	// GitHub echoes that script into the log above the step's own output. Only
	// the branch that ran is a real failure, so anything else Detect reports
	// came from reading the script rather than the run.
	detections := Detect(runLogs)

	if len(detections) != 1 || detections[0].Label != "Network failure" {
		labels := make([]string, 0, len(detections))
		for _, d := range detections {
			labels = append(labels, d.Label+" <- "+d.Line)
		}

		t.Errorf("Detect reported %v, want only the signature the run emitted", labels)
	}

	config := NewFilterConfig()
	config.Level = FilterErrors

	doc, err := ExportAsMarkdown(runLogs, config)
	if err != nil {
		t.Fatalf("exporting: %v", err)
	}

	for _, want := range []string{"# demo-pipeline", "## Detected issues", "```"} {
		if !strings.Contains(doc, want) {
			t.Errorf("export is missing %q", want)
		}
	}

	if strings.Contains(doc, "^[[") {
		t.Error("the export carries terminal escapes, which a pasted issue renders verbatim")
	}
}

// TestRecorded_FixturesMatchTheRecordedShape is the guard the hand-written
// fixtures needed. Every log fixture under testdata/logs is a stand-in for
// `gh run view --log` output, so each line must carry the same
// "job\tstep\ttimestamp message" prefix a recording does. A fixture written to
// look like whatever its author pictured is how a parser passes its tests and
// finds nothing in production.
func TestRecorded_FixturesMatchTheRecordedShape(t *testing.T) {
	t.Parallel()

	c, err := ghcassette.Load(cassette(t, "passing-run"))
	if err != nil {
		t.Fatalf("loading the cassette: %v", err)
	}

	recorded, err := c.Response("run", "view", "33467036043", "--log", "--job", "99728932318")
	if err != nil {
		t.Fatalf("reading the recorded log: %v", err)
	}

	wantFields := len(strings.Split(firstLine(t, recorded), "\t"))
	if wantFields < logFieldCount {
		t.Fatalf("the recording itself has %d tab-separated fields, want %d", wantFields, logFieldCount)
	}

	dir := filepath.Join("..", "..", "testdata", "logs")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the fixture directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(dir, entry.Name())) //nolint:gosec // a fixed testdata directory
			if err != nil {
				t.Fatalf("reading the fixture: %v", err)
			}

			for i, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
				if got := len(strings.SplitN(line, "\t", logFieldCount)); got < logFieldCount {
					t.Fatalf("line %d has %d tab-separated fields, want %d: %q\n"+
						"gh prefixes every log line with the job and step; see testdata/cassettes",
						i+1, got, logFieldCount, line)
				}
			}
		})
	}
}

func firstLine(t *testing.T, s string) string {
	t.Helper()

	line, _, found := strings.Cut(s, "\n")
	if !found {
		t.Fatal("the recording has no newline, so it is not a log transcript")
	}

	return line
}
