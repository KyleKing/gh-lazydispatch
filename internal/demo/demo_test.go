package demo_test

import (
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/demo"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

func TestNewMockConfig(t *testing.T) {
	cfg := demo.NewMockConfig()

	if cfg.Owner != "demo-org" {
		t.Errorf("Owner = %q, want %q", cfg.Owner, "demo-org")
	}
	if cfg.Repo != "demo-repo" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "demo-repo")
	}
	if cfg.Executor == nil {
		t.Error("Executor should not be nil")
	}
}

func TestSetupMockExecutor(t *testing.T) {
	cfg := demo.NewMockConfig()
	cfg.SetupMockExecutor()

	if err := logs.CheckGHCLIAvailableWithExecutor(cfg.Executor); err != nil {
		t.Errorf("gh CLI should be available in mock: %v", err)
	}
}

func TestMockExecutor_WorkflowRun(t *testing.T) {
	cfg := demo.NewMockConfig()
	cfg.SetupMockExecutor()

	client, err := github.NewClientWithExecutor("demo-org/demo-repo", cfg.Executor)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	run, err := client.GetWorkflowRun(1001)
	if err != nil {
		t.Fatalf("GetWorkflowRun failed: %v", err)
	}

	if run.Status != github.StatusCompleted {
		t.Errorf("run.Status = %q, want %q", run.Status, github.StatusCompleted)
	}
	if run.Conclusion != github.ConclusionSuccess {
		t.Errorf("run.Conclusion = %q, want %q", run.Conclusion, github.ConclusionSuccess)
	}
}

func TestMockExecutor_RunJobs(t *testing.T) {
	cfg := demo.NewMockConfig()
	cfg.SetupMockExecutor()

	client, err := github.NewClientWithExecutor("demo-org/demo-repo", cfg.Executor)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	jobs, err := client.GetWorkflowRunJobs(1001)
	if err != nil {
		t.Fatalf("GetWorkflowRunJobs failed: %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}

	if jobs[0].Name != "build" {
		t.Errorf("jobs[0].Name = %q, want %q", jobs[0].Name, "build")
	}
	if jobs[1].Name != "test" {
		t.Errorf("jobs[1].Name = %q, want %q", jobs[1].Name, "test")
	}
}

func TestDemoWorkflows(t *testing.T) {
	workflows := demo.DemoWorkflows()

	if len(workflows) != 4 {
		t.Errorf("expected 4 demo workflows, got %d", len(workflows))
	}

	names := map[string]bool{}
	for _, w := range workflows {
		names[w.Name] = true
	}

	expectedNames := []string{"CI", "Deploy", "Release", "Benchmark"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("missing expected workflow: %s", name)
		}
	}

	var deployInputs map[string]interface{}
	for _, w := range workflows {
		if w.Name == "Deploy" {
			deployInputs = make(map[string]interface{})
			for k, v := range w.GetInputs() {
				deployInputs[k] = v
			}
			break
		}
	}

	if deployInputs == nil {
		t.Fatal("Deploy workflow not found")
	}

	if len(deployInputs) != 2 {
		t.Errorf("Deploy should have 2 inputs, got %d", len(deployInputs))
	}
}

func TestInstallUninstall(t *testing.T) {
	cfg := demo.NewMockConfig()
	cfg.SetupMockExecutor()

	cfg.Install()
	defer cfg.Uninstall()
}
