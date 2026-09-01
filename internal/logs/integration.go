package logs

import (
	"fmt"
	"strconv"

	"github.com/kyleking/aragonite/cache"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
)

// LogFetcher defines the interface for fetching logs.
type LogFetcher interface {
	FetchStepLogs(runID int64, workflow string) ([]*StepLogs, error)
}

// Manager coordinates log fetching, caching, and access.
type Manager struct {
	fetcher    LogFetcher
	cache      *cache.TTLCache[[]*StepLogs]
	useRealAPI bool
}

// NewManager creates a new log manager that uses gh CLI if available.
func NewManager(client GitHubClient) *Manager {
	var fetcher LogFetcher

	useRealAPI := false

	// Try to use GHFetcher if gh CLI is available
	if err := CheckGHCLIAvailable(); err == nil {
		ghFetcher := NewGHFetcher(client)
		fetcher = &ghFetcherAdapter{ghFetcher: ghFetcher}
		useRealAPI = true
	} else {
		// Fall back to synthetic logs
		fetcher = NewFetcher(client)
	}

	return &Manager{
		fetcher:    fetcher,
		cache:      newLogCache(),
		useRealAPI: useRealAPI,
	}
}

// ghFetcherAdapter adapts GHFetcher to LogFetcher interface.
type ghFetcherAdapter struct {
	ghFetcher *GHFetcher
}

func (a *ghFetcherAdapter) FetchStepLogs(runID int64, workflow string) ([]*StepLogs, error) {
	logs, err := a.ghFetcher.FetchStepLogsReal(runID, workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch real logs via gh CLI: %w", err)
	}

	return logs, nil
}

// GetLogsForChain fetches or retrieves cached logs for a chain execution.
// Per-step fetch errors are recorded on the affected step rather than returned, but the
// error return is kept for symmetry with GetLogsForRun since callers assign both into one var.
//
//nolint:unparam // see comment above
func (m *Manager) GetLogsForChain(chainState chain.ChainState, branch string) (*RunLogs, error) {
	runLogs := NewRunLogs(chainState.ChainName, branch)

	// Fetch logs for each completed step
	for idx, result := range chainState.StepResults {
		stepLogs, err := m.stepLogs(result.RunID, result.Workflow)
		if err != nil {
			// Store error but continue with other steps
			runLogs.AddStep(&StepLogs{
				StepIndex: idx,
				Workflow:  result.Workflow,
				RunID:     result.RunID,
				Error:     err,
			})

			continue
		}

		// Add all step logs from this workflow run
		for _, sl := range stepLogs {
			sl.StepIndex = idx // Override with chain step index
			runLogs.AddStep(sl)
		}
	}

	return runLogs, nil
}

// GetLogsForRun fetches logs for a single workflow run.
func (m *Manager) GetLogsForRun(runID int64, workflow string) (*RunLogs, error) {
	// A run reached by ID carries no workflow name, and the viewer titles itself
	// from this, so the ID stands in rather than leaving the title blank.
	name := workflow
	if name == "" {
		name = "run " + strconv.FormatInt(runID, 10)
	}

	runLogs := NewRunLogs(name, "")

	stepLogs, err := m.stepLogs(runID, workflow)
	if err != nil {
		return nil, err
	}

	for _, sl := range stepLogs {
		runLogs.AddStep(sl)
	}

	return runLogs, nil
}

// stepLogs reads a run's steps, serving a copy already fetched in this session
// rather than downloading a log again. Reopening the same run is the common
// move: a reader diagnoses a failure, leaves for the timeline, and comes back.
func (m *Manager) stepLogs(runID int64, workflow string) ([]*StepLogs, error) {
	key := logCacheKey(runID, workflow)
	if cached, ok := m.cache.Get(key, cache.NoStamp); ok {
		return cached, nil
	}

	fetched, err := m.fetcher.FetchStepLogs(runID, workflow)
	if err != nil {
		return nil, fmt.Errorf("fetching the log of run %d: %w", runID, err)
	}

	m.cache.Set(key, cache.NoStamp, fetched)

	return fetched, nil
}
