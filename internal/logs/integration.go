package logs

import (
	"github.com/kyleking/gh-lazydispatch/internal/chain"
)

// Manager coordinates log fetching, caching, and access.
type Manager struct {
	fetcher *Fetcher
	cache   *Cache
}

// NewManager creates a new log manager.
func NewManager(client GitHubClient, cacheDir string) *Manager {
	return &Manager{
		fetcher: NewFetcher(client),
		cache:   NewCache(cacheDir),
	}
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
