package logs

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// StreamPollInterval is the interval between log polling for active runs.
const StreamPollInterval = 2 * time.Second

// StreamState tracks the state of logs for incremental updates.
type StreamState struct {
	StepLineCounts map[int]int // map[stepIndex]lineCount
}

// NewStreamState creates a new StreamState.
func NewStreamState() *StreamState {
	return &StreamState{
		StepLineCounts: make(map[int]int),
	}
}

// StreamUpdate represents new log content detected during streaming.
type StreamUpdate struct {
	RunID      int64
	Status     string
	Conclusion string
	NewSteps   []*StepLogs
	Error      error
}

// LogStreamer polls for incremental log updates from active workflow runs.
type LogStreamer struct {
	fetcher   *GHFetcher
	client    GitHubClient
	runID     int64
	workflow  string
	state     *StreamState
	updates   chan StreamUpdate
	ctx       context.Context
	cancel    context.CancelFunc
	ticker    *time.Ticker
	stopOnce  sync.Once
	wg        sync.WaitGroup
	mu        sync.Mutex
}

// NewLogStreamer creates a new LogStreamer for a specific run.
func NewLogStreamer(client GitHubClient, runID int64, workflow string) *LogStreamer {
	ctx, cancel := context.WithCancel(context.Background())
	return &LogStreamer{
		fetcher:  NewGHFetcher(client),
		client:   client,
		runID:    runID,
		workflow: workflow,
		state:    NewStreamState(),
		updates:  make(chan StreamUpdate, 50),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins polling for log updates.
func (s *LogStreamer) Start() {
	s.ticker = time.NewTicker(StreamPollInterval)
	s.wg.Add(1)
	go s.pollLoop()
}

// Updates returns the channel for receiving log updates.
func (s *LogStreamer) Updates() <-chan StreamUpdate {
	return s.updates
}

// Stop stops the streamer and cleans up resources.
// Safe to call multiple times.
func (s *LogStreamer) Stop() {
	s.stopOnce.Do(func() {
		s.cancel()
		if s.ticker != nil {
			s.ticker.Stop()
		}
		s.wg.Wait()
		close(s.updates)
	})
}

func (s *LogStreamer) pollLoop() {
	defer s.wg.Done()

	// Initial poll
	s.poll()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.ticker.C:
			s.poll()
		}
	}
}

func (s *LogStreamer) poll() {
	// Check run status first
	run, err := s.client.GetWorkflowRun(s.runID)
	if err != nil {
		s.sendUpdate(StreamUpdate{
			RunID: s.runID,
			Error: err,
		})
		return
	}

	// Stop polling if run completed
	if run.Status == github.StatusCompleted {
		s.sendUpdate(StreamUpdate{
			RunID:      s.runID,
			Status:     run.Status,
			Conclusion: run.Conclusion,
		})
		// Stop after sending completion update
		go s.Stop()
		return
	}

	// Fetch current logs
	currentLogs, err := s.fetcher.FetchStepLogsReal(s.runID, s.workflow)
	if err != nil {
		s.sendUpdate(StreamUpdate{
			RunID:  s.runID,
			Status: run.Status,
			Error:  err,
		})
		return
	}

	// Detect new logs
	newSteps := s.detectNewLogs(currentLogs)

	// Always send status update (NewSteps may be nil if no changes)
	s.sendUpdate(StreamUpdate{
		RunID:    s.runID,
		Status:   run.Status,
		NewSteps: newSteps,
	})
}

// detectNewLogs compares current logs against last known state.
// Returns only steps with new log lines.
func (s *LogStreamer) detectNewLogs(currentLogs []*StepLogs) []*StepLogs {
	s.mu.Lock()
	defer s.mu.Unlock()

	var newSteps []*StepLogs

	for _, stepLog := range currentLogs {
		currentLineCount := len(stepLog.Entries)
		lastLineCount := s.state.StepLineCounts[stepLog.StepIndex]

		if currentLineCount > lastLineCount {
			// New lines detected - create a StepLogs with only new entries
			newEntries := stepLog.Entries[lastLineCount:]

			newStepLog := &StepLogs{
				StepIndex:  stepLog.StepIndex,
				Workflow:   stepLog.Workflow,
				RunID:      stepLog.RunID,
				JobName:    stepLog.JobName,
				StepName:   stepLog.StepName,
				Status:     stepLog.Status,
				Conclusion: stepLog.Conclusion,
				Entries:    newEntries,
				FetchedAt:  time.Now(),
				Error:      stepLog.Error,
			}

			newSteps = append(newSteps, newStepLog)
			s.state.StepLineCounts[stepLog.StepIndex] = currentLineCount
		}
	}

	return newSteps
}

func (s *LogStreamer) sendUpdate(update StreamUpdate) {
	select {
	case <-s.ctx.Done():
		return
	case s.updates <- update:
	default:
		log.Printf("warning: log streamer update channel full, update dropped for run %d", update.RunID)
	}
}
