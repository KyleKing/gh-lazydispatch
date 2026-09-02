package github

import (
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/cache"
	"github.com/kyleking/aragonite/ghcassette"

	"github.com/kyleking/gh-lazydispatch/internal/exec"
)

// cassetteDir is where the cassettes live, resolved before any test moves the
// process: a test that chdirs to borrow another repository's remote would
// otherwise resolve testdata against wherever it went.
//
//nolint:gochecknoglobals // set once by TestMain, before any test runs
var cassetteDir string

func TestMain(m *testing.M) {
	dir, err := filepath.Abs(filepath.Join("testdata", "cassettes"))
	if err != nil {
		panic("resolving the cassette directory: " + err.Error())
	}

	cassetteDir = dir

	code := m.Run()

	ghcassette.RemoveStub()
	os.Exit(code)
}

// The repository the cassette was recorded against, and one completed run of a
// dispatch-only demo workflow on it. Re-recording reads history, so it creates
// nothing; the run IDs belong to workflows that exist to be dispatched and are
// never removed.
const (
	recordedRepo     = "KyleKing/gh-lazydispatch"
	recordedWorkflow = "demo-test.yml"
	recordedRun      = 33467036043
)

// recordedPRRepo is a different repository because this one has no open pull
// request of its own, and a recording of an empty list tests nothing. It holds
// a pull request that exists to be recorded against and carries a real check
// rollup, which is what this read is for. Re-recording against a repository
// whose pull requests carry no checks fails rather than passing on nothing.
const recordedPRRepo = "KyleKing/second-look"

// standIn makes a checkout whose only remote is repo and moves the process into
// it. A pull request listing resolves its repository from the working
// directory's remote rather than from anything the client passes, so this is the
// only way to point that read at a repository other than the one the tests live
// in.
func standIn(t *testing.T, repo string) {
	t.Helper()

	dir := t.TempDir()
	t.Chdir(dir)

	for _, args := range [][]string{
		{"init", "--initial-branch=main"},
		{"remote", "add", "origin", "https://github.com/" + repo + ".git"},
	} {
		cmd := osexec.CommandContext(t.Context(), "git", args...) //#nosec G204 -- fixed argument lists
		cmd.Dir = dir

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// recordedClient points this process's gh calls at the named cassette and
// returns a client wired to the real executor, so every read under test parses
// the bytes GitHub sent rather than a fixture of what someone thought it sends.
func recordedClient(t *testing.T, name, repo string) (*Client, *ghcassette.Session) {
	t.Helper()

	s := ghcassette.Start(t, filepath.Join(cassetteDir, name+".golden"))
	s.Apply(t)

	// A read cached in this process would not reach the cassette, so a replay
	// would pass while playing nothing.
	cache.ClearAll()

	client, err := NewClientWithExecutor(repo, exec.NewRealExecutor())
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}

	return client, s
}

// Every read this client performs, against one recording. The fields each one
// fills come from GitHub's own JSON, so a schema that moves shows up here
// rather than as an empty pane.
//
//nolint:paralleltest // Apply and Chdir move the process
func TestRecorded_EveryReadParsesWhatGitHubSent(t *testing.T) {
	client, s := recordedClient(t, "reads", recordedRepo)

	run, err := client.GetWorkflowRun(recordedRun)
	if err != nil {
		t.Fatalf("reading the run: %v", err)
	}

	if run.ID != recordedRun || run.Name == "" || run.HeadBranch == "" {
		t.Errorf("the run parsed as %+v", run)
	}

	if run.CreatedAt.IsZero() {
		t.Error("the run has no creation time, so its timestamp was not parsed")
	}

	jobs, err := client.GetWorkflowRunJobs(recordedRun)
	if err != nil {
		t.Fatalf("reading the run's jobs: %v", err)
	}

	if len(jobs) == 0 {
		t.Fatal("the run parsed with no jobs, so the timeline would draw nothing")
	}

	assertJobsCarryTheirSteps(t, jobs)

	latest, err := client.GetLatestRun(recordedWorkflow)
	if err != nil {
		t.Fatalf("reading the workflow's latest run: %v", err)
	}

	if latest.ID == 0 {
		t.Error("the latest run has no ID")
	}

	runs, err := client.ListRuns(RunQuery{Workflow: recordedWorkflow, Limit: 5})
	if err != nil {
		t.Fatalf("listing runs: %v", err)
	}

	if len(runs) == 0 {
		t.Error("the listing came back empty")
	}

	s.RequireAllPlayed(t)
}

// The timeline lays a job out by its steps' own timings, so a job whose steps
// parsed without them draws one flat bar.
func assertJobsCarryTheirSteps(t *testing.T, jobs []Job) {
	t.Helper()

	steps := 0

	for i := range jobs {
		job := &jobs[i]
		if job.Name == "" || job.StartedAt.IsZero() {
			t.Errorf("job %+v parsed without a name or a start", job)
		}

		for _, step := range job.Steps {
			steps++

			if step.Name == "" || step.Number == 0 {
				t.Errorf("step %+v parsed without a name or a number", step)
			}

			if step.StartedAt.IsZero() {
				t.Errorf("step %q has no start, so the timeline cannot place it", step.Name)
			}
		}
	}

	if steps == 0 {
		t.Error("no job carried any steps")
	}
}

// A pull request's own check rollup is what the Runs tab reads in a pull
// request scope, because runs are keyed by branch and one page of a
// repository's runs is filled by whichever branch ran last. The read is
// `gh pr list`, which the mutation guard blocks unless it tells list from
// create.
//
//nolint:paralleltest // Apply and Chdir move the process
func TestRecorded_PullRequestScopesCarryTheirChecks(t *testing.T) {
	standIn(t, recordedPRRepo)

	client, s := recordedClient(t, "pull-requests", recordedPRRepo)

	rollups := 0

	for _, scope := range []PRScope{PRScopeMine, PRScopeReviewing} {
		prs, err := client.PullRequestsInScope(scope)
		if err != nil {
			t.Fatalf("searching %q: %v", scope, err)
		}

		rollups += assertRollupsAddUp(t, prs)
	}

	// A scope matching nothing is an answer rather than an error, which is what
	// lets the pane say so. Every scope matching nothing is a recording that
	// only proves gh ran.
	if rollups == 0 {
		t.Error("no pull request carried a check rollup, so nothing here parsed one")
	}

	s.RequireAllPlayed(t)
}

// assertRollupsAddUp returns how many of prs carried a rollup, so the caller can
// tell an empty scope from a recording that parsed nothing.
func assertRollupsAddUp(t *testing.T, prs []PullRequest) int {
	t.Helper()

	withChecks := 0

	for i := range prs {
		pr := &prs[i]
		if pr.Number == 0 || pr.HeadRef == "" {
			t.Errorf("a pull request parsed as %+v", pr)
		}

		checks := pr.Checks
		if checks.Total == 0 {
			continue
		}

		withChecks++

		if got := checks.Passing + checks.Failing + checks.Pending + checks.Skipped; got != checks.Total {
			t.Errorf("#%d rolls up %d checks out of %d", pr.Number, got, checks.Total)
		}
	}

	return withChecks
}

// The environments endpoint answers with a JSON envelope, and the client reads
// it through --jq, so what it parses is a bare name per line. A repository
// declaring none answers with an empty list rather than an error, which is what
// keeps an environment input on free text instead of an empty picker.
//
//nolint:paralleltest // Apply and Chdir move the process
func TestRecorded_EnvironmentsAreNamesOnePerLine(t *testing.T) {
	client, s := recordedClient(t, "environments", recordedRepo)

	names, err := client.ListEnvironments()
	if err != nil {
		t.Fatalf("reading the environments: %v", err)
	}

	// The envelope leaking through --jq is the failure worth naming: a name
	// carrying a brace or a quote is JSON that was never unwrapped.
	for _, name := range names {
		if name != strings.TrimSpace(name) || strings.ContainsAny(name, `{}"`) {
			t.Errorf("%q is not a bare environment name", name)
		}
	}

	s.RequireAllPlayed(t)
}
