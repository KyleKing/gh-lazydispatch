# Log Viewer Integration Guide

## Overview

This document shows how to integrate the multi-step log viewer into the lazydispatch application.

## Architecture

### Components

```
┌─────────────────────────────────────────────────────────────┐
│                     Application Layer                       │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  App Model (internal/app/)                            │  │
│  │  - Handles LogViewerMsg                               │  │
│  │  - Manages LogsViewerModal lifecycle                  │  │
│  │  - Coordinates log fetching                           │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Log Management Layer                      │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  LogManager (internal/logs/)                          │  │
│  │  - Coordinates fetching and caching                   │  │
│  │  - Provides unified interface                         │  │
│  └───────────────────────────────────────────────────────┘  │
│                            │                                 │
│         ┌──────────────────┴──────────────────┐             │
│         ↓                                      ↓             │
│  ┌─────────────┐                      ┌──────────────┐      │
│  │   Fetcher   │                      │    Cache     │      │
│  │  (GitHub    │                      │  (Disk-based │      │
│  │   API)      │                      │   storage)   │      │
│  └─────────────┘                      └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                            │
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                        UI Layer                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  LogsViewerModal (internal/ui/modal/)                 │  │
│  │  - Multi-step tabs                                    │  │
│  │  - Search and filter                                  │  │
│  │  - Syntax highlighting                                │  │
│  │  - Match navigation                                   │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Integration Steps

### Step 1: Add LogManager to App Model

**File:** `internal/app/app.go`

```go
type Model struct {
	// ... existing fields ...

	ghClient    *github.Client
	watcher     *watcher.Watcher
	logManager  *logs.Manager  // NEW
}

func NewModel() Model {
	// ... existing initialization ...

	// Initialize log manager
	cacheDir := filepath.Join(os.UserCacheDir(), "lazydispatch", "logs")
	var logManager *logs.Manager
	if ghClient != nil {
		logManager = logs.NewManager(ghClient, cacheDir)
		logManager.LoadCache() // Load cached logs on startup
	}

	return Model{
		// ... existing fields ...
		logManager: logManager,
	}
}
```

### Step 2: Add Message Types

**File:** `internal/app/messages.go` (create if doesn't exist)

```go
package app

import (
	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

// FetchLogsMsg requests fetching logs for a chain or run.
type FetchLogsMsg struct {
	ChainState *chain.ChainState // If from chain
	RunID      int64             // If from single run
	Workflow   string            // If from single run
	Branch     string
	ErrorsOnly bool // Pre-filter for errors
}

// LogsFetchedMsg contains fetched logs or an error.
type LogsFetchedMsg struct {
	Logs       *logs.RunLogs
	ErrorsOnly bool
	Error      error
}

// ShowLogsViewerMsg opens the logs viewer modal.
type ShowLogsViewerMsg struct {
	Logs       *logs.RunLogs
	ErrorsOnly bool
}
```

### Step 3: Add Key Binding for Log Viewer

**File:** `internal/app/keymap.go`

```go
type keyMap struct {
	// ... existing bindings ...

	ViewLogs key.Binding  // NEW: 'L' to view logs
}

func defaultKeyMap() keyMap {
	return keyMap{
		// ... existing bindings ...

		ViewLogs: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "view logs"),
		),
	}
}
```

### Step 4: Add Message Handlers

**File:** `internal/app/handlers.go`

```go
// Add to Update() method
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// ... existing cases ...

	case FetchLogsMsg:
		return m, m.fetchLogs(msg)

	case LogsFetchedMsg:
		if msg.Error != nil {
			m.error = msg.Error
			return m, nil
		}
		return m, func() tea.Msg {
			return ShowLogsViewerMsg{
				Logs:       msg.Logs,
				ErrorsOnly: msg.ErrorsOnly,
			}
		}

	case ShowLogsViewerMsg:
		return m.showLogsViewer(msg.Logs, msg.ErrorsOnly), nil

	// Handle logs viewer modal updates
	case tea.KeyMsg:
		if m.modal != nil {
			if logsViewer, ok := m.modal.(*modal.LogsViewerModal); ok {
				newModal, cmd := logsViewer.Update(msg)
				if logsViewer.IsDone() {
					m.modal = nil
					return m, nil
				}
				m.modal = newModal
				return m, cmd
			}
		}
	}

	// ... existing handlers ...
}

// fetchLogs creates a command to fetch logs asynchronously.
func (m Model) fetchLogs(msg FetchLogsMsg) tea.Cmd {
	return func() tea.Msg {
		if m.logManager == nil {
			return LogsFetchedMsg{
				Error: fmt.Errorf("log manager not initialized"),
			}
		}

		var runLogs *logs.RunLogs
		var err error

		if msg.ChainState != nil {
			// Fetch logs for chain execution
			runLogs, err = m.logManager.GetLogsForChain(*msg.ChainState, msg.Branch)
		} else if msg.RunID != 0 {
			// Fetch logs for single run
			runLogs, err = m.logManager.GetLogsForRun(msg.RunID, msg.Workflow)
		} else {
			return LogsFetchedMsg{
				Error: fmt.Errorf("no chain state or run ID provided"),
			}
		}

		return LogsFetchedMsg{
			Logs:       runLogs,
			ErrorsOnly: msg.ErrorsOnly,
			Error:      err,
		}
	}
}

// showLogsViewer creates and displays the logs viewer modal.
func (m Model) showLogsViewer(runLogs *logs.RunLogs, errorsOnly bool) Model {
	width := m.width
	height := m.height

	if errorsOnly {
		m.modal = modal.NewLogsViewerModalWithError(runLogs, width, height)
	} else {
		m.modal = modal.NewLogsViewerModal(runLogs, width, height)
	}

	return m
}
```

### Step 5: Integrate with ChainStatusModal

**File:** `internal/ui/modal/chain_status.go`

Add key binding to view logs when chain completes or fails:

```go
type chainStatusKeyMap struct {
	Close    key.Binding
	Stop     key.Binding
	Copy     key.Binding
	OpenURL  key.Binding
	ViewLogs key.Binding  // NEW
}

func defaultChainStatusKeyMap() chainStatusKeyMap {
	return chainStatusKeyMap{
		// ... existing bindings ...
		ViewLogs: key.NewBinding(key.WithKeys("l")),
	}
}

// In Update() method:
case key.Matches(msg, m.keys.ViewLogs):
	if m.state.Status == chain.ChainCompleted || m.state.Status == chain.ChainFailed {
		// Request log fetch
		errorsOnly := m.state.Status == chain.ChainFailed
		return m, func() tea.Msg {
			return FetchLogsMsg{
				ChainState: &m.state,
				Branch:     m.branch,
				ErrorsOnly: errorsOnly,
			}
		}
	}

// Update help text in View():
if m.state.Status == chain.ChainCompleted || m.state.Status == chain.ChainFailed {
	s.WriteString(ui.HelpStyle.Render("[esc/q] close  [l] view logs  [c] copy script"))
} else {
	s.WriteString(ui.HelpStyle.Render("[esc/q] close (continues)  [C-c] stop  [c] copy script"))
}
```

### Step 6: Add History Integration

**File:** `internal/ui/panes/history.go`

Add ability to view logs from history entries:

```go
// In history pane's Update() method:
case tea.KeyMsg:
	switch msg.String() {
	case "l": // View logs for selected entry
		if m.selectedIdx >= 0 && m.selectedIdx < len(m.entries) {
			entry := m.entries[m.selectedIdx]
			if entry.ChainName != "" {
				// Reconstruct chain state from history
				return m, func() tea.Msg {
					chainState := reconstructChainStateFromHistory(entry)
					return FetchLogsMsg{
						ChainState: &chainState,
						Branch:     entry.Branch,
						ErrorsOnly: false,
					}
				}
			}
		}
	}
}

// Helper to reconstruct chain state
func reconstructChainStateFromHistory(entry frecency.HistoryEntry) chain.ChainState {
	// Parse metadata to reconstruct step results
	// This requires enhancing history entries to store step metadata
	return chain.ChainState{
		ChainName:    entry.ChainName,
		// ... reconstruct other fields ...
	}
}
```

## Usage Patterns

### Pattern 1: View Logs After Chain Completes

```go
// User flow:
// 1. Execute chain
// 2. Chain completes successfully
// 3. Press 'l' in ChainStatusModal
// 4. Logs viewer opens showing all steps
// 5. User can tab between steps, search, filter
```

### Pattern 2: View Error Logs After Failure

```go
// User flow:
// 1. Execute chain
// 2. Chain fails at step 2
// 3. Press 'l' in ChainStatusModal
// 4. Logs viewer opens pre-filtered to errors
// 5. Only shows step 2 logs with error entries
// 6. User can see exactly what failed
```

### Pattern 3: Review Historical Execution

```go
// User flow:
// 1. Navigate to History pane
// 2. Select previous chain execution
// 3. Press 'l' to view logs
// 4. Logs loaded from cache (if available)
// 5. Can review what happened in past run
```

### Pattern 4: Compare Logs Across Steps

```go
// User flow:
// 1. Open logs viewer
// 2. Use tab to switch between steps
// 3. Use '/' to search for specific term
// 4. See which steps mention the term
// 5. Identify patterns across execution
```

### Pattern 5: Debug Specific Error

```go
// User flow:
// 1. Open logs with error filter
// 2. See all error entries across steps
// 3. Press 'n' to jump to next error
// 4. Press 'N' to jump to previous error
// 5. Quickly scan all failure points
```

## Enhanced History Storage

To support log viewing from history, enhance the history entry structure:

**File:** `internal/frecency/types.go`

```go
type HistoryEntry struct {
	// ... existing fields ...

	ChainName   string                `json:"chain_name,omitempty"`
	StepResults []HistoryStepResult   `json:"step_results,omitempty"` // NEW
}

type HistoryStepResult struct {
	StepIndex  int               `json:"step_index"`
	Workflow   string            `json:"workflow"`
	RunID      int64             `json:"run_id"`
	Status     string            `json:"status"`
	Conclusion string            `json:"conclusion"`
	Inputs     map[string]string `json:"inputs,omitempty"`
}
```

Update chain execution to store step results:

**File:** `internal/app/handlers.go`

```go
// When chain completes:
case ChainUpdateMsg:
	// ... existing handling ...

	if msg.Update.State.Status == chain.ChainCompleted ||
	   msg.Update.State.Status == chain.ChainFailed {
		// Store in history with step results
		entry := frecency.HistoryEntry{
			Timestamp:   time.Now(),
			ChainName:   msg.Update.State.ChainName,
			// ... other fields ...
			StepResults: convertStepResults(msg.Update.State.StepResults),
		}
		m.history.Add(entry)
	}
}

func convertStepResults(results map[int]*chain.StepResult) []frecency.HistoryStepResult {
	var historyResults []frecency.HistoryStepResult
	for idx, result := range results {
		historyResults = append(historyResults, frecency.HistoryStepResult{
			StepIndex:  idx,
			Workflow:   result.Workflow,
			RunID:      result.RunID,
			Status:     string(result.Status),
			Conclusion: result.Conclusion,
			Inputs:     result.Inputs,
		})
	}
	return historyResults
}
```

## Real-Time Log Streaming (Future Enhancement)

For real-time log streaming during chain execution:

```go
// Add to ChainExecutor
func (e *ChainExecutor) StreamLogs(runID int64) <-chan logs.LogEntry {
	logChan := make(chan logs.LogEntry, 100)

	go func() {
		defer close(logChan)

		// Poll for new logs periodically
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		lastSeenLine := 0

		for {
			select {
			case <-e.stopCh:
				return
			case <-ticker.C:
				// Fetch logs and send new entries
				newEntries := fetchLogsAfterLine(runID, lastSeenLine)
				for _, entry := range newEntries {
					logChan <- entry
					lastSeenLine++
				}
			}
		}
	}()

	return logChan
}

// In LogsViewerModal, handle streaming updates:
type LogStreamUpdateMsg struct {
	Entry logs.LogEntry
}

func (m *LogsViewerModal) subscribeToStream(logChan <-chan logs.LogEntry) tea.Cmd {
	return func() tea.Msg {
		entry := <-logChan
		return LogStreamUpdateMsg{Entry: entry}
	}
}
```

## Testing

### Unit Tests

```go
// Test log fetching
func TestFetcher_FetchStepLogs(t *testing.T) {
	// Mock GitHub client
	// Test successful fetch
	// Test error handling
}

// Test filtering
func TestFilter_Apply(t *testing.T) {
	// Test error filter
	// Test search term
	// Test regex matching
}

// Test cache
func TestCache_GetPut(t *testing.T) {
	// Test cache hit
	// Test cache miss
	// Test expiration
}
```

### Integration Tests

```go
// Test end-to-end log viewing
func TestLogViewer_Integration(t *testing.T) {
	// Create test chain execution
	// Fetch logs
	// Verify modal displays correctly
	// Test tab switching
	// Test filtering
	// Test search
}
```

## Performance Considerations

### Caching Strategy

```go
// Cache TTL: 1 hour for completed runs, 5 minutes for active runs
const (
	CompletedRunTTL = 1 * time.Hour
	ActiveRunTTL    = 5 * time.Minute
)

// Cache size limit: 100 MB
const MaxCacheSize = 100 * 1024 * 1024
```

### Pagination

For very large log files:

```go
// Load logs in chunks
type PaginatedLogs struct {
	Entries    []LogEntry
	Page       int
	PageSize   int
	TotalCount int
	HasMore    bool
}

// Viewport only renders visible portion
```

### Background Cleanup

```go
// In app initialization:
go func() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if m.logManager != nil {
			m.logManager.ClearExpired()
		}
	}
}()
```

## Next Steps

1. Implement real log fetching via `gh run view --log`
2. Add log export functionality
3. Implement match navigation (n/N keys)
4. Add syntax highlighting for common log formats
5. Support log streaming for active runs
6. Add log annotations and bookmarks
7. Implement side-by-side comparison view
