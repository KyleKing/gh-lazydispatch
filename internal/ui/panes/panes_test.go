package panes

import (
	"testing"
	"time"

	"github.com/kyleking/gh-wfr/internal/frecency"
	"github.com/kyleking/gh-wfr/internal/workflow"
)

func testWorkflows() []workflow.WorkflowFile {
	return []workflow.WorkflowFile{
		{
			Name:     "Deploy",
			Filename: "deploy.yml",
			On: workflow.OnTrigger{
				WorkflowDispatch: &workflow.WorkflowDispatch{
					Inputs: map[string]workflow.WorkflowInput{
						"environment": {
							Type:    "choice",
							Default: "staging",
							Options: []string{"production", "staging"},
						},
						"dry_run": {
							Type:    "boolean",
							Default: "false",
						},
					},
				},
			},
		},
		{
			Name:     "CI",
			Filename: "ci.yml",
			On: workflow.OnTrigger{
				WorkflowDispatch: &workflow.WorkflowDispatch{},
			},
		},
	}
}

func TestWorkflowModel_SelectedWorkflow(t *testing.T) {
	m := NewWorkflowModel(testWorkflows())
	m.SetSize(40, 20)

	wf := m.SelectedWorkflow()
	if wf == nil {
		t.Fatal("expected non-nil workflow")
	}
	if wf.Filename != "deploy.yml" {
		t.Errorf("expected 'deploy.yml', got %q", wf.Filename)
	}
}

func TestWorkflowItem_FilterValue(t *testing.T) {
	wf := workflow.WorkflowFile{Name: "Deploy", Filename: "deploy.yml"}
	item := WorkflowItem{workflow: wf}

	fv := item.FilterValue()
	if fv != "Deploy deploy.yml" {
		t.Errorf("expected 'Deploy deploy.yml', got %q", fv)
	}
}

func TestHistoryModel_SetEntries(t *testing.T) {
	m := NewHistoryModel()
	m.SetSize(60, 20)

	entries := []frecency.HistoryEntry{
		{Workflow: "deploy.yml", Branch: "main", LastRunAt: time.Now()},
		{Workflow: "deploy.yml", Branch: "feature", LastRunAt: time.Now().Add(-1 * time.Hour)},
	}

	m.SetEntries(entries, "deploy.yml")

	entry := m.SelectedEntry()
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.Branch != "main" {
		t.Errorf("expected branch 'main', got %q", entry.Branch)
	}
}

func TestHistoryItem_Description(t *testing.T) {
	item := HistoryItem{
		entry: frecency.HistoryEntry{
			Branch:    "main",
			Inputs:    map[string]string{"env": "prod"},
			LastRunAt: time.Now().Add(-30 * time.Minute),
		},
	}

	desc := item.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestConfigModel_SetWorkflow(t *testing.T) {
	m := NewConfigModel()
	m.SetSize(80, 20)

	wfs := testWorkflows()
	m.SetWorkflow(&wfs[0])

	names := m.GetInputNames()
	if len(names) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(names))
	}

	envVal := m.GetInputValue("environment")
	if envVal != "staging" {
		t.Errorf("expected default 'staging', got %q", envVal)
	}
}

func TestConfigModel_BuildCommand(t *testing.T) {
	m := NewConfigModel()
	wfs := testWorkflows()
	m.SetWorkflow(&wfs[0])
	m.SetBranch("main")
	m.SetInput("environment", "production")

	cmd := m.BuildCommand()
	if len(cmd) < 3 {
		t.Fatalf("expected at least 3 args, got %d", len(cmd))
	}

	if cmd[0] != "workflow" || cmd[1] != "run" || cmd[2] != "deploy.yml" {
		t.Errorf("unexpected command prefix: %v", cmd[:3])
	}

	hasRef := false
	hasEnv := false
	for i, arg := range cmd {
		if arg == "--ref" && i+1 < len(cmd) && cmd[i+1] == "main" {
			hasRef = true
		}
		if arg == "-f" && i+1 < len(cmd) && cmd[i+1] == "environment=production" {
			hasEnv = true
		}
	}

	if !hasRef {
		t.Error("expected --ref main in command")
	}
	if !hasEnv {
		t.Error("expected -f environment=production in command")
	}
}

func TestConfigModel_ToggleWatchRun(t *testing.T) {
	m := NewConfigModel()

	if m.WatchRun() {
		t.Error("expected watchRun to be false initially")
	}

	m.ToggleWatchRun()
	if !m.WatchRun() {
		t.Error("expected watchRun to be true after toggle")
	}

	m.ToggleWatchRun()
	if m.WatchRun() {
		t.Error("expected watchRun to be false after second toggle")
	}
}
