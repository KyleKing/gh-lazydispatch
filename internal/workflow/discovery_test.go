package workflow_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

func TestDiscover(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to determine current file")
	}
	repoRoot := filepath.Join(filepath.Dir(currentFile), "..", "..", "testdata")

	workflows, err := workflow.Discover(repoRoot)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(workflows) != 12 {
		t.Errorf("expected 12 dispatchable workflows, got %d", len(workflows))

		for _, wf := range workflows {
			t.Logf("  found: %s", wf.Filename)
		}
	}

	filenames := make(map[string]bool)
	for _, wf := range workflows {
		filenames[wf.Filename] = true
	}

	if filenames["ci.yml"] {
		t.Error("ci.yml should not be included (not dispatchable)")
	}

	if filenames["not-dispatchable.yml"] {
		t.Error("not-dispatchable.yml should not be included")
	}
}

func TestDiscover_NonExistentDir(t *testing.T) {
	t.Parallel()

	workflows, err := workflow.Discover("/nonexistent/path")
	if err != nil {
		t.Fatalf("Discover should not error on missing dir: %v", err)
	}

	if len(workflows) != 0 {
		t.Errorf("expected 0 workflows for missing dir, got %d", len(workflows))
	}
}

func TestDiscover_EmptyDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, ".github", "workflows"), 0o750); err != nil {
		t.Fatal(err)
	}

	workflows, err := workflow.Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(workflows) != 0 {
		t.Errorf("expected 0 workflows for empty dir, got %d", len(workflows))
	}
}
