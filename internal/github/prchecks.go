package github

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// Check buckets, as `gh pr checks` reports them.
const (
	CheckPass    = "pass"
	CheckFail    = "fail"
	CheckPending = "pending"
	CheckSkip    = "skipping"
	CheckCancel  = "cancel"
)

// PRCheck is one check on a pull request.
type PRCheck struct {
	Name     string `json:"name"`
	Bucket   string `json:"bucket"`
	Link     string `json:"link"`
	Workflow string `json:"workflow"`
}

var runLink = regexp.MustCompile(`/actions/runs/(\d+)`)

// RunID is the Actions run the check belongs to, or 0 for a check from another
// app, whose link does not point at a run.
func (c PRCheck) RunID() int64 {
	m := runLink.FindStringSubmatch(c.Link)
	if m == nil {
		return 0
	}

	id, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0
	}

	return id
}

// GetPRChecks lists the checks on a pull request. The gh CLI exits non-zero while
// checks are pending or failing, so a non-empty JSON answer wins over the exit
// status.
func (c *Client) GetPRChecks(number int) ([]PRCheck, error) {
	stdout, stderr, err := c.executor.Execute("gh", "pr", "checks", strconv.Itoa(number),
		"--repo", c.fullName(), "--json", "name,bucket,link,workflow")

	var checks []PRCheck
	if jsonErr := json.Unmarshal([]byte(stdout), &checks); jsonErr != nil {
		if err != nil {
			return nil, fmt.Errorf("reading the checks of pull request %d: %w (stderr: %s)", number, err, stderr)
		}

		return nil, fmt.Errorf("parsing the checks of pull request %d: %w", number, jsonErr)
	}

	return checks, nil
}
