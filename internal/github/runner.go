package github

import (
	"context"
	"fmt"

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
