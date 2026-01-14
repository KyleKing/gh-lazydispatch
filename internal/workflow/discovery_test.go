package workflow

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiscover(t *testing.T) {
	_, currentFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(currentFile), "..", "..", "testdata")

	workflows, err := Discover(repoRoot)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(workflows) != 8 {
		t.Errorf("expected 8 dispatchable workflows, got %d", len(workflows))
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
	workflows, err := Discover("/nonexistent/path")
	if err != nil {
		t.Fatalf("Discover should not error on missing dir: %v", err)
	}

	if len(workflows) != 0 {
		t.Errorf("expected 0 workflows for missing dir, got %d", len(workflows))
	}
}

func TestDiscover_EmptyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "workflow-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.MkdirAll(filepath.Join(tmpDir, ".github", "workflows"), 0755); err != nil {
		t.Fatal(err)
	}

	workflows, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(workflows) != 0 {
		t.Errorf("expected 0 workflows for empty dir, got %d", len(workflows))
	}
}
