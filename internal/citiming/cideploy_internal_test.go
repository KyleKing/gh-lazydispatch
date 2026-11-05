package citiming

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

// A deploy is measured from its commit time to its up step, its smoke run is
// found by SHA or else by start time, and a repeat run on the deployed SHA
// shows as more than one post-deploy run per deploy.
func TestSummarizeDeploys_MeasuresEachPhaseAndCountsPostDeployRuns(t *testing.T) {
	t.Parallel()

	at := func(minutes int) time.Time { return time.Date(2026, 10, 1, 12, minutes, 0, 0, time.UTC) }

	deploys := []github.WorkflowRunInfo{
		{ID: 1, HeadSHA: "a", Conclusion: "success", CreatedAt: at(2), CommittedAt: at(0)},
		{ID: 2, HeadSHA: "b", Conclusion: "success", CreatedAt: at(40), CommittedAt: at(30)},
	}

	jobs := runcache.Jobs{
		1: {{Name: "deploy", Steps: []github.StepTiming{
			{Name: "Run Pulumi deploy", StartedAt: at(5), CompletedAt: at(15)},
		}}},
		2: {{Name: "deploy", Steps: []github.StepTiming{
			{Name: "Run Pulumi deploy", StartedAt: at(45), CompletedAt: at(50)},
		}}},
	}

	smoke := []github.WorkflowRunInfo{
		{HeadSHA: "a", Conclusion: "success", CreatedAt: at(17), RunStartedAt: at(18), UpdatedAt: at(28)},
		{HeadSHA: "other", Conclusion: "success", CreatedAt: at(52), RunStartedAt: at(52), UpdatedAt: at(56)},
	}

	checks := []github.WorkflowRunInfo{
		{HeadSHA: "a", Event: "workflow_dispatch", CreatedAt: at(16)},
		{HeadSHA: "a", Event: "workflow_dispatch", CreatedAt: at(20)},
		{HeadSHA: "a", Event: "pull_request", CreatedAt: at(21)},
		{HeadSHA: "b", Event: "workflow_dispatch", CreatedAt: at(51)},
	}

	got := summarizeDeploys("deploy-stage.yml", deploys, jobs, smoke,
		map[string][]github.WorkflowRunInfo{"checks.yml": checks}, "pulumi deploy", at(59))

	want := map[string]float64{
		"merge to up": got.MergeToUp.P50Seconds, "up": got.Up.P50Seconds,
		"up to smoke": got.UpToSmoke.P50Seconds, "smoke": got.Smoke.P50Seconds,
	}

	expected := map[string]float64{"merge to up": 10 * 60, "up": 7.5 * 60, "up to smoke": 2.5 * 60, "smoke": 7 * 60}
	for name, seconds := range expected {
		if want[name] != seconds {
			t.Errorf("%s P50 = %v, want %v", name, want[name], seconds)
		}
	}

	if len(got.PostDeploy) != 1 || got.PostDeploy[0].Runs != 3 || got.PostDeploy[0].Repeated != 1 {
		t.Errorf("post-deploy = %+v, want 3 runs over 2 deploys with 1 repeated", got.PostDeploy)
	}

	var md strings.Builder

	writeDeploys(&md, []ciDeploy{got})

	if !strings.Contains(md.String(), "| checks.yml | 3 | 1.50 | 1 |") {
		t.Errorf("markdown missing the per-deploy count:\n%s", md.String())
	}
}
