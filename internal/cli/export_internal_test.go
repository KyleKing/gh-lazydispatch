package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// runLocal drives a command that reads the repository rather than GitHub, from
// the root of the given fixture tree.
func runLocal(t *testing.T, root string, args ...string) (string, string, int) {
	t.Helper()
	t.Chdir(root)

	var stdout, stderr bytes.Buffer

	code := Run(append([]string{"export"}, args...), &stdout, &stderr)

	return stdout.String(), stderr.String(), code
}

func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}

	return root
}

// TestExportWorkflows_DescribesEveryDispatchableInput is what an agent reads
// instead of opening each workflow YAML, so it has to carry enough to build a
// dispatch: the filename, and every input's type, default, and options.
//
//nolint:paralleltest // t.Chdir rules out t.Parallel
func TestExportWorkflows_DescribesEveryDispatchableInput(t *testing.T) {
	stdout, stderr, code := runLocal(t, repoRoot(t), "workflows")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	var found []workflowSummary
	if err := json.Unmarshal([]byte(stdout), &found); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}

	byFile := make(map[string]workflowSummary, len(found))
	for _, summary := range found {
		byFile[summary.Filename] = summary
	}

	demo, ok := byFile["demo-chain-check.yml"]
	if !ok {
		t.Fatalf("demo-chain-check.yml is missing from %v", byFile)
	}

	mode, ok := demo.Inputs["fail_mode"]
	if !ok {
		t.Fatalf("fail_mode is missing from %v", demo.Inputs)
	}

	if mode.Type != "choice" || len(mode.Options) == 0 || mode.Default == "" {
		t.Errorf("fail_mode is %+v, want a choice with options and a default", mode)
	}

	if _, listed := byFile["ci.yml"]; listed {
		t.Error("ci.yml is not dispatchable and should not be listed")
	}

	if !strings.Contains(stderr, "dispatchable workflows") {
		t.Errorf("stderr does not report a count: %q", stderr)
	}
}

// TestExportChains_KeepsStepOrder matters because a chain is a sequence: a
// caller reading it back has to see the steps in the order they dispatch.
//
//nolint:paralleltest // t.Chdir rules out t.Parallel
func TestExportChains_KeepsStepOrder(t *testing.T) {
	stdout, stderr, code := runLocal(t, repoRoot(t), "chains")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	var found []chainSummary
	if err := json.Unmarshal([]byte(stdout), &found); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}

	if len(found) == 0 {
		t.Fatal("no chains, but .github/lazydispatch.yml defines some")
	}

	for _, chain := range found {
		if chain.Name == "" {
			t.Error("a chain came back with no name")
		}

		if len(chain.Steps) == 0 {
			t.Errorf("chain %q has no steps", chain.Name)
		}

		for _, step := range chain.Steps {
			if step.Workflow == "" {
				t.Errorf("chain %q has a step with no workflow", chain.Name)
			}
		}
	}
}

//nolint:paralleltest // t.Chdir rules out t.Parallel
func TestExportChains_ReportsAMissingConfigRatherThanCrashing(t *testing.T) {
	_, stderr, code := runLocal(t, t.TempDir(), "chains")
	if code != 1 {
		t.Fatalf("exit %d, want 1 for a directory with no config", code)
	}

	if !strings.Contains(stderr, "lazydispatch.yml") {
		t.Errorf("stderr does not name the file it looked for: %q", stderr)
	}
}

func TestRun_RejectsUnknownCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{"no command", nil},
		{"not export", []string{"dispatch"}},
		{"no export command", []string{"export"}},
		{"unknown export command", []string{"export", "everything"}},
		// --current reduces a branch's runs to one per workflow, so without a
		// branch it would silently answer about the whole repository.
		{"current without a branch", []string{"export", "runs", "--current"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			if code := Run(tt.args, &stdout, &stderr); code != exitUsage {
				t.Errorf("Run(%v) = %d, want %d", tt.args, code, exitUsage)
			}

			if stdout.Len() != 0 {
				t.Errorf("usage went to stdout: %q", stdout.String())
			}

			if !strings.Contains(stderr.String(), "export diagnose") {
				t.Errorf("usage does not list the commands: %q", stderr.String())
			}
		})
	}
}
