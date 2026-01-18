package github

import "time"

// WorkflowRun represents a GitHub Actions workflow run.
type WorkflowRun struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	HTMLURL    string    `json:"html_url"`
	HeadBranch string    `json:"head_branch"`
}

// RunStatus constants
const (
	StatusQueued     = "queued"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
)

// Conclusion constants
const (
	ConclusionSuccess   = "success"
	ConclusionFailure   = "failure"
	ConclusionCancelled = "cancelled"
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
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	StartedAt  time.Time `json:"started_at"`
	Steps      []Step    `json:"steps"`
}

// Step represents a step within a job.
type Step struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Number     int    `json:"number"`
}

// JobsResponse represents the API response for listing jobs.
type JobsResponse struct {
	TotalCount int   `json:"total_count"`
	Jobs       []Job `json:"jobs"`
}

// RunsResponse represents the API response for listing runs.
type RunsResponse struct {
	TotalCount   int           `json:"total_count"`
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}
