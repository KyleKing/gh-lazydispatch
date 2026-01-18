package watcher

import (
	"context"
	"sync"
	"time"

	"github.com/kyleking/lazydispatch/internal/github"
)

// PollInterval is the default interval between API polls.
const PollInterval = 5 * time.Second

// WatchedRun represents a run being watched.
type WatchedRun struct {
	RunID      int64
	Workflow   string
	Status     string
	Conclusion string
	Jobs       []JobStatus
	HTMLURL    string
	UpdatedAt  time.Time
}

// JobStatus represents the status of a job in a watched run.
type JobStatus struct {
	Name       string
	Status     string
	Conclusion string
	Steps      []StepStatus
}

// StepStatus represents the status of a step in a job.
type StepStatus struct {
	Name       string
	Status     string
	Conclusion string
	Number     int
}

// IsActive returns true if the run is still in progress.
func (r WatchedRun) IsActive() bool {
	return r.Status == github.StatusQueued || r.Status == github.StatusInProgress
}

// IsSuccess returns true if the run completed successfully.
func (r WatchedRun) IsSuccess() bool {
	return r.Status == github.StatusCompleted && r.Conclusion == github.ConclusionSuccess
}

// RunUpdate represents an update to a watched run.
type RunUpdate struct {
	RunID int64
	Run   WatchedRun
	Error error
}

// RunWatcher monitors workflow runs and sends updates.
type RunWatcher struct {
	client     *github.Client
	runs       map[int64]*WatchedRun
	updates    chan RunUpdate
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	ticker     *time.Ticker
	isPolling  bool
	pollingMu  sync.Mutex
}

// NewWatcher creates a new RunWatcher.
func NewWatcher(client *github.Client) *RunWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &RunWatcher{
		client:  client,
		runs:    make(map[int64]*WatchedRun),
		updates: make(chan RunUpdate, 100),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Watch starts watching a workflow run.
func (w *RunWatcher) Watch(runID int64, workflowName string) {
	w.mu.Lock()
	w.runs[runID] = &WatchedRun{
		RunID:    runID,
		Workflow: workflowName,
		Status:   github.StatusQueued,
	}
	w.mu.Unlock()

	w.ensurePolling()
	w.pollRun(runID)
}

// Unwatch stops watching a workflow run.
func (w *RunWatcher) Unwatch(runID int64) {
	w.mu.Lock()
	delete(w.runs, runID)
	w.mu.Unlock()
}

// Updates returns the channel for receiving run updates.
func (w *RunWatcher) Updates() <-chan RunUpdate {
	return w.updates
}

// GetRuns returns all currently watched runs.
func (w *RunWatcher) GetRuns() []WatchedRun {
	w.mu.RLock()
	defer w.mu.RUnlock()

	runs := make([]WatchedRun, 0, len(w.runs))
	for _, run := range w.runs {
		runs = append(runs, *run)
	}
	return runs
}

// GetRun returns a specific watched run.
func (w *RunWatcher) GetRun(runID int64) (*WatchedRun, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	run, ok := w.runs[runID]
	if !ok {
		return nil, false
	}
	return run, true
}

// ActiveCount returns the number of active runs.
func (w *RunWatcher) ActiveCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	count := 0
	for _, run := range w.runs {
		if run.IsActive() {
			count++
		}
	}
	return count
}

// TotalCount returns the total number of watched runs.
func (w *RunWatcher) TotalCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.runs)
}

// Stop stops the watcher and cleans up resources.
func (w *RunWatcher) Stop() {
	w.cancel()
	if w.ticker != nil {
		w.ticker.Stop()
	}
	close(w.updates)
}

// ClearCompleted removes all completed runs from the watch list.
func (w *RunWatcher) ClearCompleted() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for id, run := range w.runs {
		if !run.IsActive() {
			delete(w.runs, id)
		}
	}
}

func (w *RunWatcher) ensurePolling() {
	w.pollingMu.Lock()
	defer w.pollingMu.Unlock()

	if w.isPolling {
		return
	}
	w.isPolling = true

	w.ticker = time.NewTicker(PollInterval)
	go w.pollLoop()
}

func (w *RunWatcher) pollLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.ticker.C:
			w.pollAllRuns()
		}
	}
}

func (w *RunWatcher) pollAllRuns() {
	w.mu.RLock()
	runIDs := make([]int64, 0, len(w.runs))
	for id, run := range w.runs {
		if run.IsActive() {
			runIDs = append(runIDs, id)
		}
	}
	w.mu.RUnlock()

	for _, id := range runIDs {
		w.pollRun(id)
	}
}

func (w *RunWatcher) pollRun(runID int64) {
	run, err := w.client.GetWorkflowRun(runID)
	if err != nil {
		w.sendUpdate(RunUpdate{RunID: runID, Error: err})
		return
	}

	jobs, err := w.client.GetWorkflowRunJobs(runID)
	if err != nil {
		w.sendUpdate(RunUpdate{RunID: runID, Error: err})
		return
	}

	watched := WatchedRun{
		RunID:      runID,
		Workflow:   run.Name,
		Status:     run.Status,
		Conclusion: run.Conclusion,
		HTMLURL:    run.HTMLURL,
		UpdatedAt:  run.UpdatedAt,
		Jobs:       make([]JobStatus, len(jobs)),
	}

	for i, job := range jobs {
		watched.Jobs[i] = JobStatus{
			Name:       job.Name,
			Status:     job.Status,
			Conclusion: job.Conclusion,
			Steps:      make([]StepStatus, len(job.Steps)),
		}
		for j, step := range job.Steps {
			watched.Jobs[i].Steps[j] = StepStatus{
				Name:       step.Name,
				Status:     step.Status,
				Conclusion: step.Conclusion,
				Number:     step.Number,
			}
		}
	}

	w.mu.Lock()
	w.runs[runID] = &watched
	w.mu.Unlock()

	w.sendUpdate(RunUpdate{RunID: runID, Run: watched})
}

func (w *RunWatcher) sendUpdate(update RunUpdate) {
	select {
	case w.updates <- update:
	default:
	}
}
