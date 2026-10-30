package github

import (
	"testing"
	"time"
)

// A pull request that never queued reads back as null, and the queue events of
// the others keep their order and reasons.
func TestParseQueueEvents_KeepsReasonsAndToleratesNullPullRequests(t *testing.T) {
	t.Parallel()

	payload := `{"data":{"repository":{
		"p1":{"timelineItems":{"nodes":[
			{"__typename":"AddedToMergeQueueEvent","createdAt":"2026-01-01T00:00:00Z"},
			{"__typename":"RemovedFromMergeQueueEvent","createdAt":"2026-01-01T00:02:00Z","reason":"failed_checks"}]}},
		"p2":null}}}`

	out := map[int][]QueueEvent{}
	if err := parseQueueEvents(payload, []int{1, 2}, out); err != nil {
		t.Fatal(err)
	}

	if got := out[1]; len(got) != 2 || got[0].Type != "added" || got[1].Reason != "failed_checks" ||
		got[1].At.Sub(got[0].At) != 2*time.Minute {
		t.Errorf("events = %+v, want an addition then a failed_checks removal two minutes later", got)
	}

	if events, ok := out[2]; !ok || len(events) != 0 {
		t.Errorf("null pull request = %v (present %v), want an empty entry", events, ok)
	}
}
