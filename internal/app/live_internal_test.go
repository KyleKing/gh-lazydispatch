//go:build live

// This file is excluded from the normal test build. It is compiled only with
// `-tags live` and is driven by scripts/live-test.sh, which creates and destroys
// the throwaway GitHub repository it dispatches against. Running it by hand
// without that setup is a no-op: it skips unless the harness env vars are set.
package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// scratchRepoPrefix guards against ever dispatching into a repo that the
// harness did not create. Nothing in this file runs unless the repo name
// carries it.
const scratchRepoPrefix = "gh-ld-livetest-"

const (
	dispatchWaitTimeout = 90 * time.Second
	runPollTimeout      = 120 * time.Second
	runPollInterval     = 3 * time.Second
)

type liveEnv struct {
	repo     string
	dir      string
	workflow string
	branch   string
	nonce    string
	result   string
}

func loadLiveEnv(t *testing.T) liveEnv {
	t.Helper()

	env := liveEnv{
		repo:     os.Getenv("GH_LD_LIVE_REPO"),
		dir:      os.Getenv("GH_LD_LIVE_DIR"),
		workflow: os.Getenv("GH_LD_LIVE_WORKFLOW"),
		branch:   os.Getenv("GH_LD_LIVE_BRANCH"),
		nonce:    os.Getenv("GH_LD_LIVE_NONCE"),
		result:   os.Getenv("GH_LD_LIVE_RESULT"),
	}

	if env.repo == "" {
		t.Skip("GH_LD_LIVE_REPO unset; run this test through scripts/live-test.sh")
	}

	_, name, ok := strings.Cut(env.repo, "/")
	if !ok || !strings.HasPrefix(name, scratchRepoPrefix) {
		t.Fatalf("refusing to dispatch: repo %q is not a %q scratch repo", env.repo, scratchRepoPrefix)
	}

	for field, value := range map[string]string{
		"GH_LD_LIVE_BRANCH":   env.branch,
		"GH_LD_LIVE_DIR":      env.dir,
		"GH_LD_LIVE_NONCE":    env.nonce,
		"GH_LD_LIVE_RESULT":   env.result,
		"GH_LD_LIVE_WORKFLOW": env.workflow,
	} {
		if value == "" {
			t.Fatalf("%s is unset", field)
		}
	}

	return env
}

// TestLiveDispatch drives the TUI headlessly against a throwaway repo and lets
// the real dispatch path run: the same tea.ExecProcess call the binary makes,
// shelling out to a real `gh workflow run`.
func TestLiveDispatch(t *testing.T) {
	env := loadLiveEnv(t)

	// Keep frecency history out of the real user profile.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Chdir(env.dir)

	workflows, _, err := workflow.Discover(env.dir)
	if err != nil {
		t.Fatalf("discovering workflows in %s: %v", env.dir, err)
	}

	assertScratchWorkflow(t, workflows, env.workflow)

	want := map[string]string{
		"level":   "warn",
		"message": "live-" + env.nonce,
		"verbose": "true",
	}

	tm := teatest.NewTestModel(
		t,
		New(workflows, frecency.NewStore(), env.repo),
		teatest.WithInitialTermSize(200, 50),
	)

	driveToConfirmation(t, tm, want)
	waitForOutput(t, tm, "Confirm Workflow Execution")

	tm.Send(tea.KeyPressMsg{Code: 'y', Text: "y"})

	// gh workflow run writes its success line to the program's stdout, which is
	// the teatest buffer. Waiting on it also orders the quit key after the
	// dispatch, which would otherwise race the confirmation command.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Created workflow_dispatch event"))
	}, teatest.WithDuration(dispatchWaitTimeout))

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(dispatchWaitTimeout))

	final, ok := tm.FinalModel(t).(Model)
	if !ok {
		t.Fatalf("final model is %T, want app.Model", tm.FinalModel(t))
	}

	assertBuiltCommand(t, final, env, want)
	assertHistoryRecorded(t, final, env, want)

	run := waitForDispatchedRun(t, env)
	t.Logf("dispatched run %d: %s", run.DatabaseID, run.URL)

	writeResult(t, env, run, want)
}

func assertScratchWorkflow(t *testing.T, workflows []workflow.File, filename string) {
	t.Helper()

	for _, wf := range workflows {
		if wf.Filename != filename {
			continue
		}

		inputs := wf.GetInputs()

		for name, wantType := range map[string]string{
			"level":   "choice",
			"message": "string",
			"verbose": "boolean",
		} {
			input, found := inputs[name]
			if !found {
				t.Fatalf("workflow %s is missing input %q", filename, name)
			}

			if got := input.InputType(); got != wantType {
				t.Errorf("input %q has type %q, want %q", name, got, wantType)
			}
		}

		return
	}

	t.Fatalf("workflow %s not discovered; found %d dispatchable workflows", filename, len(workflows))
}

// driveToConfirmation replays the keystrokes a user would type: focus the
// config pane, set each of the three input types through its modal, then open
// the run confirmation.
//
// Inputs are numbered in sorted order, so level=0, message=1, verbose=2.
//
// A modal's confirm key returns a tea.Cmd that delivers the result message from
// a goroutine. Sending the next keystroke immediately races that goroutine: if
// the key lands first, the next modal is already on the stack and swallows the
// pending result. Each step therefore waits until the new value appears in the
// config pane's command preview before moving on.
func driveToConfirmation(t *testing.T, tm *teatest.TestModel, want map[string]string) {
	t.Helper()

	waitForOutput(t, tm, "livetest")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})

	// level: choice modal, default "info", move down once to "warn".
	tm.Send(tea.KeyPressMsg{Code: '0', Text: "0"})
	waitForOutput(t, tm, "[↑↓] navigate")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "-f level="+want["level"])

	// message: text modal, clear the default then type the nonce value.
	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	waitForOutput(t, tm, "Message echoed back")
	tm.Send(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})

	for _, r := range want["message"] {
		tm.Send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "-f message="+want["message"])

	// verbose: boolean modal, "y" selects true.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	waitForOutput(t, tm, "[y/n] quick")
	tm.Send(tea.KeyPressMsg{Code: 'y', Text: "y"})
	waitForOutput(t, tm, "-f verbose="+want["verbose"])

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// assertBuiltCommand checks the command the TUI constructed. BuildArgs ranges
// over a map, so the flag order is not stable and each fragment is checked
// independently.
func assertBuiltCommand(t *testing.T, m Model, env liveEnv, want map[string]string) {
	t.Helper()

	got := m.buildCLIString()

	fragments := []string{
		"gh workflow run " + env.workflow,
		"--ref " + env.branch,
	}
	for name, value := range want {
		fragments = append(fragments, "-f "+name+"="+value)
	}

	sort.Strings(fragments)

	for _, fragment := range fragments {
		if !strings.Contains(got, fragment) {
			t.Errorf("built command %q is missing %q", got, fragment)
		}
	}
}

func assertHistoryRecorded(t *testing.T, m Model, env liveEnv, want map[string]string) {
	t.Helper()

	entries := m.history.TopForRepo(env.repo, env.workflow, 1)
	if len(entries) == 0 {
		t.Fatal("dispatch was not recorded in frecency history")
	}

	entry := entries[0]
	if entry.Branch != env.branch {
		t.Errorf("history branch is %q, want %q", entry.Branch, env.branch)
	}

	for name, value := range want {
		if got := entry.Inputs[name]; got != value {
			t.Errorf("history input %q is %q, want %q", name, got, value)
		}
	}
}

type ghRun struct {
	Event      string `json:"event"`
	HeadBranch string `json:"headBranch"`
	Status     string `json:"status"`
	URL        string `json:"url"`
	DatabaseID int64  `json:"databaseId"`
}

func waitForDispatchedRun(t *testing.T, env liveEnv) ghRun {
	t.Helper()

	deadline := time.Now().Add(runPollTimeout)

	for {
		runs := listRuns(t, env)
		for _, run := range runs {
			if run.Event == "workflow_dispatch" && run.HeadBranch == env.branch {
				return run
			}
		}

		if time.Now().After(deadline) {
			t.Fatalf("no workflow_dispatch run appeared for %s on %s within %s", env.workflow, env.repo, runPollTimeout)
		}

		time.Sleep(runPollInterval)
	}
}

func listRuns(t *testing.T, env liveEnv) []ghRun {
	t.Helper()

	//nolint:noctx // short-lived read-only gh call inside a live test
	cmd := exec.Command(
		"gh", "run", "list",
		"--repo", env.repo,
		"--workflow", env.workflow,
		"--limit", "10",
		"--json", "databaseId,event,headBranch,status,url",
	)

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Logf("gh run list failed (retrying): %v: %s", err, stderr.String())
		return nil
	}

	var runs []ghRun
	if err := json.Unmarshal(stdout.Bytes(), &runs); err != nil {
		t.Fatalf("parsing gh run list output %q: %v", stdout.String(), err)
	}

	return runs
}

// writeResult hands the run id and the values the TUI sent to the shell
// harness, which verifies them against what the workflow echoed back.
func writeResult(t *testing.T, env liveEnv, run ghRun, want map[string]string) {
	t.Helper()

	var b strings.Builder

	fmt.Fprintf(&b, "run_id=%d\n", run.DatabaseID)
	fmt.Fprintf(&b, "run_url=%s\n", run.URL)

	names := make([]string, 0, len(want))
	for name := range want {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		fmt.Fprintf(&b, "input_%s=%s\n", name, want[name])
	}

	if err := os.MkdirAll(filepath.Dir(env.result), 0o750); err != nil {
		t.Fatalf("creating result dir: %v", err)
	}

	if err := os.WriteFile(env.result, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("writing %s: %v", env.result, err)
	}
}
