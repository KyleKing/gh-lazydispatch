package github

import (
	"context"
	"fmt"

	ghforge "github.com/kyleking/aragonite/forge/github"
)

// QueueEvent is one merge queue event on a pull request's timeline.
type QueueEvent = ghforge.QueueEvent

// Merge queue event types.
const (
	QueueAdded   = ghforge.QueueAdded
	QueueRemoved = ghforge.QueueRemoved
	QueueMerged  = ghforge.QueueMerged
)

// ListMergeQueueEvents reads the merge queue events of pull requests, which the
// REST API does not expose.
func (c *Client) ListMergeQueueEvents(numbers []int) (map[int][]QueueEvent, error) {
	events, err := ghforge.MergeQueueEvents(c.runnerContext(context.Background()), "", c.fullName(), numbers)
	if err != nil {
		return nil, fmt.Errorf("listing merge queue events: %w", err)
	}

	return events, nil
}
