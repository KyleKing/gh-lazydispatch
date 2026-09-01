package github

import "github.com/kyleking/aragonite/forge"

// Job is one job of a workflow run, as aragonite models it.
type Job = forge.Job

// Step is one step of a job, as aragonite models it.
type Step = forge.Step

// WorkflowRun is one Actions run, as aragonite models it. Every read in this
// package goes through aragonite, so a local struct would only be a conversion
// of this one.
type WorkflowRun = forge.WorkflowRun

// RunStatus constants.
const (
	StatusQueued     = "queued"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
)

// Conclusion constants.
const (
	ConclusionSuccess   = "success"
	ConclusionFailure   = "failure"
	ConclusionCancelled = "cancelled" //nolint:misspell // matches GitHub Actions API's actual conclusion value
	ConclusionSkipped   = "skipped"
)

// JobsResponse represents the API response for listing jobs.
type JobsResponse struct {
	Jobs       []Job `json:"jobs"`
	TotalCount int   `json:"total_count"`
}
