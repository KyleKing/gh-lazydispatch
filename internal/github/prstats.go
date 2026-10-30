package github

import (
	"encoding/json"
	"fmt"
	"time"
)

// PRStats is the size of a pull request, which decides how much CI it should cost.
type PRStats struct {
	CreatedAt    time.Time `json:"created_at"`
	Title        string    `json:"title"`
	Author       string    `json:"author"`
	Number       int       `json:"number"`
	Commits      int       `json:"commits"`
	ChangedFiles int       `json:"changed_files"`
	Additions    int       `json:"additions"`
	Deletions    int       `json:"deletions"`
}

// GetPRStats reads one pull request's size.
func (c *Client) GetPRStats(number int) (PRStats, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d", c.fullName(), number)

	stdout, stderr, err := c.executor.Execute("gh", "api", path)
	if err != nil {
		return PRStats{}, fmt.Errorf("reading pull request %d: %w (stderr: %s)", number, err, stderr)
	}

	var payload struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		PRStats
	}

	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return PRStats{}, fmt.Errorf("parsing pull request %d: %w", number, err)
	}

	payload.Author = payload.User.Login

	return payload.PRStats, nil
}
