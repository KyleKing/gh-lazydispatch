package github

import (
	"fmt"
)

// JobLog fetches one job's raw log. The text carries GitHub's escape
// sequences, which gh refuses to print unless told it is not a terminal.
func (c *Client) JobLog(jobID int64) (string, error) {
	path := fmt.Sprintf("repos/%s/actions/jobs/%d/logs", c.fullName(), jobID)

	stdout, stderr, err := c.executor.Execute("gh", "api", path, "--allow-escape-sequences")
	if err != nil {
		return "", fmt.Errorf("reading the log of job %d: %w (stderr: %s)", jobID, err, stderr)
	}

	return stdout, nil
}
