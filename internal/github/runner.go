package github

import (
	"context"
	"fmt"

	"github.com/kyleking/aragonite/forge"
	ghforge "github.com/kyleking/aragonite/forge/github"

	"github.com/kyleking/gh-lazydispatch/internal/exec"
)

// runnerContext returns a context that routes aragonite's gh calls through this
// client's executor.
//
// Every gh invocation has to pass through internal/exec, which is where the
// mutation guard lives: it panics rather than let a test reach
// `gh workflow run`. A read moved into aragonite that called gh directly would
// leave that guard covering only the calls that had not moved yet, so the seam
// travels with the call rather than being reinstated at each one.
func (c *Client) runnerContext(ctx context.Context) context.Context {
	return ghforge.WithRunner(ctx, executorRunner(c.executor))
}

func executorRunner(executor exec.CommandExecutor) ghforge.Runner {
	return func(_ context.Context, _ string, _ []string, args ...string) ([]byte, error) {
		stdout, stderr, err := executor.Execute("gh", args...)
		if err != nil {
			return nil, fmt.Errorf("gh api failed: %w (stderr: %s)", err, stderr)
		}

		return []byte(stdout), nil
	}
}

// fullName is the "owner/repo" aragonite's Actions reads take.
func (c *Client) fullName() string {
	return c.owner + "/" + c.repo
}

// fromForge converts aragonite's run model to this package's. The two differ
// only in the URL field's name, which the TUI and the export commands both
// spell as the API does.
func fromForge(run forge.WorkflowRun) WorkflowRun {
	return WorkflowRun{
		ID:         run.ID,
		Name:       run.Name,
		Status:     run.Status,
		Conclusion: run.Conclusion,
		HTMLURL:    run.URL,
		HeadBranch: run.HeadBranch,
		Event:      run.Event,
		Path:       run.Path,
		CreatedAt:  run.CreatedAt,
		UpdatedAt:  run.UpdatedAt,
	}
}

func fromForgeRuns(runs []forge.WorkflowRun) []WorkflowRun {
	out := make([]WorkflowRun, 0, len(runs))
	for i := range runs {
		out = append(out, fromForge(runs[i]))
	}

	return out
}

func fromForgeJobs(jobs []forge.Job) []Job {
	out := make([]Job, 0, len(jobs))

	for i := range jobs {
		job := &jobs[i]
		steps := make([]Step, 0, len(job.Steps))

		for _, s := range job.Steps {
			steps = append(steps, Step{
				Name: s.Name, Status: s.Status, Conclusion: s.Conclusion, Number: s.Number,
				StartedAt: s.StartedAt, CompletedAt: s.CompletedAt,
			})
		}

		out = append(out, Job{
			ID: job.ID, Name: job.Name, Status: job.Status, Conclusion: job.Conclusion,
			StartedAt: job.StartedAt, CompletedAt: job.CompletedAt, Steps: steps,
		})
	}

	return out
}
