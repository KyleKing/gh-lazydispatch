package github

import "time"

// WorkflowRun represents a GitHub Actions workflow run.
type WorkflowRun struct {
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	HTMLURL    string    `json:"html_url"`
	HeadBranch string    `json:"head_branch"`
	ID         int64     `json:"id"`
}

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

// IsActive returns true if the run is still in progress.
func (r WorkflowRun) IsActive() bool {
	return r.Status == StatusQueued || r.Status == StatusInProgress
}

// IsSuccess returns true if the run completed successfully.
func (r WorkflowRun) IsSuccess() bool {
	return r.Status == StatusCompleted && r.Conclusion == ConclusionSuccess
}

// Job represents a job within a workflow run.
type Job struct {
	StartedAt time.Time `json:"started_at"`
	// CompletedAt is the zero time while the job is still running.
	CompletedAt time.Time `json:"completed_at"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	Steps       []Step    `json:"steps"`
	ID          int64     `json:"id"`
}

// Step represents a step within a job.
type Step struct {
	StartedAt time.Time `json:"started_at"`
	// CompletedAt is the zero time while the step is still running, and
	// StartedAt is zero for a step that has not begun.
	CompletedAt time.Time `json:"completed_at"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	Number      int       `json:"number"`
}

// JobsResponse represents the API response for listing jobs.
type JobsResponse struct {
	Jobs       []Job `json:"jobs"`
	TotalCount int   `json:"total_count"`
}

// RunsResponse represents the API response for listing runs.
type RunsResponse struct {
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
	TotalCount   int           `json:"total_count"`
}
