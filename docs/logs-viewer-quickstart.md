# Log Viewer Quick Start Implementation

This guide provides copy-pastable code to get the log viewer working quickly.

## Step 1: Add Message Types

Create `internal/app/logs_messages.go`:

```go
package app

import (
	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

// FetchLogsMsg requests fetching logs for a chain or run.
type FetchLogsMsg struct {
	ChainState *chain.ChainState
	RunID      int64
	Workflow   string
	Branch     string
	ErrorsOnly bool
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

## Step 2: Update App Model

In `internal/app/app.go`, add logManager field:

```go
import (
	// ... existing imports ...
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"path/filepath"
	"os"
)

type Model struct {
	// ... existing fields ...

	ghClient    *github.Client
	watcher     *watcher.Watcher
	logManager  *logs.Manager  // ADD THIS
}

// In NewModel() or initialization function:
func initializeModel() Model {
	// ... existing initialization ...

	// Initialize GitHub client
	var ghClient *github.Client
	var logManager *logs.Manager

	if repo != "" {
		var err error
		ghClient, err = github.NewClient(repo)
		if err == nil {
			// Initialize log manager
			cacheDir := filepath.Join(os.UserCacheDir(), "lazydispatch", "logs")
			logManager = logs.NewManager(ghClient, cacheDir)
			logManager.LoadCache() // Load from disk on startup
		}
	}

	return Model{
		// ... existing fields ...
		ghClient:   ghClient,
		logManager: logManager,
	}
}
```

## Step 3: Add Message Handlers

In `internal/app/handlers.go` (or `app.go` Update method):

```go
// In Update() method, add these cases:

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
	m = m.showLogsViewer(msg.Logs, msg.ErrorsOnly)
	return m, nil

// Add these helper methods:

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
			runLogs, err = m.logManager.GetLogsForChain(*msg.ChainState, msg.Branch)
		} else if msg.RunID != 0 {
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

	var logsModal *modal.LogsViewerModal
	if errorsOnly {
		logsModal = modal.NewLogsViewerModalWithError(runLogs, width, height)
	} else {
		logsModal = modal.NewLogsViewerModal(runLogs, width, height)
	}

	m.modal = logsModal
	return m
}
```

## Step 4: Update ChainStatusModal

In `internal/ui/modal/chain_status.go`:

```go
// Add import if needed
import (
	// ... existing ...
	"github.com/kyleking/gh-lazydispatch/internal/app"
)

// Update keymap:
type chainStatusKeyMap struct {
	Close    key.Binding
	Stop     key.Binding
	Copy     key.Binding
	ViewLogs key.Binding  // ADD THIS
}

func defaultChainStatusKeyMap() chainStatusKeyMap {
	return chainStatusKeyMap{
		Close:    key.NewBinding(key.WithKeys("esc", "q")),
		Stop:     key.NewBinding(key.WithKeys("ctrl+c")),
		Copy:     key.NewBinding(key.WithKeys("c")),
		ViewLogs: key.NewBinding(key.WithKeys("l")),  // ADD THIS
	}
}

// In Update() method, add:
case key.Matches(msg, m.keys.ViewLogs):
	// Only allow viewing logs when chain is completed or failed
	if m.state.Status == chain.ChainCompleted || m.state.Status == chain.ChainFailed {
		errorsOnly := m.state.Status == chain.ChainFailed
		return m, func() tea.Msg {
			return app.FetchLogsMsg{
				ChainState: &m.state,
				Branch:     m.branch,
				ErrorsOnly: errorsOnly,
			}
		}
	}

// Update View() help text:
func (m *ChainStatusModal) View() string {
	var s strings.Builder

	// ... existing view code ...

	// Update help text to include 'l' key
	if m.state.Status == chain.ChainCompleted || m.state.Status == chain.ChainFailed {
		s.WriteString(ui.HelpStyle.Render("[esc/q] close  [l] view logs  [c] copy script"))
	} else {
		s.WriteString(ui.HelpStyle.Render("[esc/q] close (continues)  [C-c] stop  [c] copy script"))
	}

	return s.String()
}
```

## Step 5: Handle Modal Updates in App

In `internal/app/app.go` Update method:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// ... existing cases ...

	// Handle modal updates
	if m.modal != nil {
		switch currentModal := m.modal.(type) {
		case *modal.LogsViewerModal:
			newModal, cmd := currentModal.Update(msg)
			if currentModal.IsDone() {
				m.modal = nil
				return m, nil
			}
			m.modal = newModal
			return m, cmd
		// ... other modal types ...
		}
	}

	// ... rest of update logic ...
}
```

## Step 6: Build and Test

```bash
# Build
go build -o lazydispatch ./cmd/lazydispatch

# Test with a chain
./lazydispatch

# Execute a chain (or select from history)
# When it completes, press 'l' to view logs
```

## Expected Behavior

### Successful Chain
```
1. Run chain: release-bump
2. Chain completes successfully
3. Press 'l' in ChainStatusModal
4. Logs viewer opens showing all steps
5. Tab through steps: ci-gate.yml, version-bump.yml
6. Use '/' to search for specific terms
7. Use 'f' to cycle filters
8. Press 'q' to close
```

### Failed Chain
```
1. Run chain: release-bump
2. CI gate fails
3. Press 'l' in ChainStatusModal
4. Logs viewer opens pre-filtered to errors
5. Only shows ci-gate.yml (failed step)
6. Error entries highlighted in red
7. Press 'f' to see all logs (not just errors)
8. Investigate cause of failure
```

## Debugging

### Logs not appearing

Add debug output to fetchLogs:

```go
func (m Model) fetchLogs(msg FetchLogsMsg) tea.Cmd {
	return func() tea.Msg {
		log.Printf("Fetching logs: chainState=%v, runID=%d", msg.ChainState != nil, msg.RunID)

		if m.logManager == nil {
			log.Printf("ERROR: logManager is nil")
			return LogsFetchedMsg{Error: fmt.Errorf("log manager not initialized")}
		}

		var runLogs *logs.RunLogs
		var err error

		if msg.ChainState != nil {
			log.Printf("Fetching logs for chain: %s", msg.ChainState.ChainName)
			runLogs, err = m.logManager.GetLogsForChain(*msg.ChainState, msg.Branch)
		} else if msg.RunID != 0 {
			log.Printf("Fetching logs for run: %d", msg.RunID)
			runLogs, err = m.logManager.GetLogsForRun(msg.RunID, msg.Workflow)
		}

		if err != nil {
			log.Printf("ERROR fetching logs: %v", err)
		} else {
			log.Printf("Successfully fetched logs: %d steps", len(runLogs.AllSteps()))
		}

		return LogsFetchedMsg{
			Logs:       runLogs,
			ErrorsOnly: msg.ErrorsOnly,
			Error:      err,
		}
	}
}
```

Run with logging:

```bash
./lazydispatch 2> debug.log
# In another terminal:
tail -f debug.log
```

### Modal not responding to keys

Check that modal context is properly passed:

```go
// In Update(), ensure LogsViewerModal is handled
case tea.KeyMsg:
	if m.modal != nil {
		if logsViewer, ok := m.modal.(*modal.LogsViewerModal); ok {
			log.Printf("LogsViewer handling key: %s", msg.String())
			newModal, cmd := logsViewer.Update(msg)
			// ... rest of handling
		}
	}
```

### Cache not working

Check cache directory:

```bash
# macOS/Linux
ls -lah ~/Library/Caches/lazydispatch/logs/
# or
ls -lah ~/.cache/lazydispatch/logs/

# Should see .json files with timestamps
```

Clear cache if needed:

```bash
rm -rf ~/Library/Caches/lazydispatch/logs/
# or
rm -rf ~/.cache/lazydispatch/logs/
```

## Next Steps

Once basic functionality works:

1. **Add export functionality:**
   - Implement markdown export
   - Add export button to modal
   - Test with various log sizes

2. **Enhance filtering:**
   - Add regex support
   - Implement match navigation (n/N keys)
   - Add quick filters (errors, warnings)

3. **Improve performance:**
   - Add virtual scrolling for large logs
   - Implement lazy loading of step details
   - Optimize search with indexing

4. **Add features:**
   - Timeline view
   - Diff mode
   - Real-time streaming
   - Bookmarks

## Common Issues

### Issue: "log manager not initialized"

**Cause:** GitHub client failed to initialize

**Fix:**
```go
// Check that repo is detected
repo, err := runner.GetRepoName()
if err != nil {
	log.Printf("Failed to get repo: %v", err)
}

// Ensure you're in a git repo
cd /path/to/your/repo
./lazydispatch
```

### Issue: Tabs show "No logs available"

**Cause:** Fetcher returned empty logs or error

**Fix:**
```go
// In fetcher.go, add logging:
func (f *Fetcher) FetchStepLogs(runID int64, workflow string) ([]*StepLogs, error) {
	log.Printf("Fetching logs for runID=%d, workflow=%s", runID, workflow)

	jobs, err := f.client.GetWorkflowRunJobs(runID)
	if err != nil {
		log.Printf("ERROR getting jobs: %v", err)
		return nil, err
	}

	log.Printf("Found %d jobs", len(jobs))
	// ... rest
}
```

### Issue: Search not working

**Cause:** Search input not focused or filter preventing matches

**Fix:**
```go
// In LogsViewerModal.handleSearchInput:
case msg.Type == tea.KeyEnter:
	log.Printf("Applying search: %q", m.searchInput.Value())
	m.filterCfg.SearchTerm = m.searchInput.Value()
	m.applyFilter()
	log.Printf("Filter applied, found %d entries", m.filtered.TotalEntries())
```

## Performance Tips

### For Large Logs (>10,000 lines)

1. **Use filtering early:**
   ```
   Press 'f' to filter to errors only
   Use specific search terms
   ```

2. **Tab through steps instead of searching all:**
   ```
   Navigate to specific failing step
   Then search within that step
   ```

3. **Export and use external tools:**
   ```
   Press 'e' to export
   Open in editor with better search
   ```

### For Slow Network/API

1. **Enable caching:**
   ```go
   // In NewManager:
   manager.cache.TTL = 2 * time.Hour  // Longer cache
   ```

2. **Fetch on-demand:**
   ```
   Don't fetch until user presses 'l'
   Cache aggressively
   ```

## Success Checklist

- [ ] LogManager added to app.Model
- [ ] Message types defined
- [ ] Handlers wire up messages
- [ ] ChainStatusModal has 'l' key binding
- [ ] Modal updates handled in app.Update
- [ ] Logs fetch successfully
- [ ] Tabs show step names
- [ ] Filtering works (all/errors/warnings)
- [ ] Search highlights matches
- [ ] Modal closes with 'q'
- [ ] Cache persists between runs

## Resources

- **Full docs:** See `docs/README-logs.md`
- **Examples:** See `docs/logs-viewer-features.md`
- **Integration:** See `docs/logs-viewer-integration.md`
- **Architecture:** See `internal/logs/*.go`

---

**Ready to implement!** Start with Step 1 and work through sequentially. Each step builds on the previous.
