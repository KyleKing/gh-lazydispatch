package logs

import (
	"fmt"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
)

// LogFetcher defines the interface for fetching logs.
type LogFetcher interface {
	FetchStepLogs(runID int64, workflow string) ([]*StepLogs, error)
}

// Manager coordinates log fetching, caching, and access.
type Manager struct {
	fetcher    LogFetcher
	cache      *Cache
	useRealAPI bool
}

// NewManager creates a new log manager that uses gh CLI if available.
func NewManager(client GitHubClient, cacheDir string) *Manager {
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
		cache:      NewCache(cacheDir),
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
func (m *Manager) GetLogsForChain(chainState chain.ChainState, branch string) (*RunLogs, error) {
	runLogs := NewRunLogs(chainState.ChainName, branch)

	// Fetch logs for each completed step
	for idx, result := range chainState.StepResults {
		stepLogs, err := m.fetcher.FetchStepLogs(result.RunID, result.Workflow)
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
	runLogs := NewRunLogs("", "")

	stepLogs, err := m.fetcher.FetchStepLogs(runID, workflow)
	if err != nil {
		return nil, err
	}

	for _, sl := range stepLogs {
		runLogs.AddStep(sl)
	}

	return runLogs, nil
}

// LoadCache loads the log cache from disk.
func (m *Manager) LoadCache() error {
	return m.cache.Load()
}

// ClearExpired removes expired entries from the cache.
func (m *Manager) ClearExpired() error {
	return m.cache.Clear()
}
