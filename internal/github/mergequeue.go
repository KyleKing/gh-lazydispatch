package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// QueueEvent is one merge queue event on a pull request's timeline. Type is
// "added", "removed", or "merged", and Reason is set on removals.
type QueueEvent struct {
	At     time.Time
	Type   string
	Reason string
}

const timelineQuery = `p%d:pullRequest(number:%d){timelineItems(first:50,` +
	`itemTypes:[ADDED_TO_MERGE_QUEUE_EVENT,REMOVED_FROM_MERGE_QUEUE_EVENT,MERGED_EVENT]){nodes{__typename ` +
	`... on AddedToMergeQueueEvent{createdAt} ... on RemovedFromMergeQueueEvent{createdAt reason} ` +
	`... on MergedEvent{createdAt}}}} `

// mergeQueueBatch is how many pull requests one GraphQL query reads.
const mergeQueueBatch = 25

// ListMergeQueueEvents reads the merge queue events of pull requests, which the
// REST API does not expose. Pull requests with none map to an empty slice.
func (c *Client) ListMergeQueueEvents(numbers []int) (map[int][]QueueEvent, error) {
	out := make(map[int][]QueueEvent, len(numbers))

	for start := 0; start < len(numbers); start += mergeQueueBatch {
		batch := numbers[start:min(start+mergeQueueBatch, len(numbers))]

		var query strings.Builder

		fmt.Fprintf(&query, "query{repository(owner:%q,name:%q){", c.owner, c.repo)

		for _, n := range batch {
			fmt.Fprintf(&query, timelineQuery, n, n)
		}

		query.WriteString("}}")

		stdout, stderr, err := c.executor.Execute("gh", "api", "graphql", "-f", "query="+query.String())
		if err != nil {
			return nil, fmt.Errorf("reading merge queue events: %w (stderr: %s)", err, stderr)
		}

		if err := parseQueueEvents(stdout, batch, out); err != nil {
			return nil, err
		}
	}

	return out, nil
}

func parseQueueEvents(stdout string, batch []int, out map[int][]QueueEvent) error {
	var payload struct {
		Data struct {
			Repository map[string]*struct {
				TimelineItems struct {
					Nodes []struct {
						CreatedAt time.Time `json:"createdAt"`  //nolint:tagliatelle // GraphQL field name
						Type      string    `json:"__typename"` //nolint:tagliatelle // GraphQL field name
						Reason    string    `json:"reason"`
					} `json:"nodes"`
				} `json:"timelineItems"` //nolint:tagliatelle // GraphQL field name
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return fmt.Errorf("parsing merge queue events: %w", err)
	}

	kinds := map[string]string{
		"AddedToMergeQueueEvent": "added", "RemovedFromMergeQueueEvent": "removed", "MergedEvent": "merged",
	}

	for _, n := range batch {
		out[n] = nil

		pr := payload.Data.Repository[fmt.Sprintf("p%d", n)]
		if pr == nil {
			continue
		}

		for _, node := range pr.TimelineItems.Nodes {
			out[n] = append(out[n], QueueEvent{At: node.CreatedAt, Type: kinds[node.Type], Reason: node.Reason})
		}
	}

	return nil
}
