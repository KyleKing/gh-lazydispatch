# Log Viewer System - Complete Guide

## Overview

This document provides a comprehensive overview of the log viewer system for lazydispatch, including implementation, features, and integration.

## Quick Links

- **[Chain Examples](./chain-examples.md)** - Commitizen bump chain with CI gating
- **[Error Alerting](./chain-failure-alerting.md)** - Enhanced error handling design
- **[Implementation Guide](./implementation-guide.md)** - Step-by-step implementation
- **[Integration Guide](./logs-viewer-integration.md)** - How to integrate with the app
- **[Feature Ideas](./logs-viewer-features.md)** - Innovative features and proposals

## What's Included

### 1. Core Infrastructure (`internal/logs/`)

**Files Created:**
- `types.go` - Core data structures (LogEntry, StepLogs, RunLogs, FilterConfig)
- `fetcher.go` - GitHub API integration for log fetching
- `cache.go` - Disk-based log caching with TTL
- `filter.go` - Advanced filtering and search functionality
- `integration.go` - Manager for coordinating log operations

**Key Features:**
- Type-safe log structures with metadata
- Automatic log level detection (error, warning, info, debug)
- Configurable caching with expiration
- Regex and text-based search
- Filter by level, search term, or specific step

### 2. UI Components (`internal/ui/modal/`)

**Files Created:**
- `logs_viewer.go` - Multi-step log viewer modal

**Key Features:**
- Tabbed interface for step navigation
- Real-time search with highlighting
- Filter cycling (all → errors → warnings → all)
- Scrollable viewport with keyboard navigation
- Responsive to window size changes

### 3. Documentation

**Comprehensive Guides:**
- Architecture diagrams and data flow
- Integration patterns with existing code
- Usage examples and user flows
- Testing strategies
- Performance considerations

## Architecture at a Glance

```
User Action (Press 'l' on failed chain)
    ↓
FetchLogsMsg dispatched
    ↓
LogManager.GetLogsForChain()
    ├─ Check cache first
    ├─ Fetch from GitHub API if needed
    └─ Parse and structure logs
    ↓
LogsFetchedMsg with RunLogs
    ↓
ShowLogsViewerMsg
    ↓
LogsViewerModal created and displayed
    ├─ Tabs show each step
    ├─ Initial filter applied (errors if from failure)
    ├─ User can search, filter, navigate
    └─ Export or share logs
```

## Implementation Phases

### Phase 1: Foundation (Completed in Design)
✅ Core data structures
✅ Fetcher with GitHub API integration
✅ Cache system with TTL
✅ Filter engine with search
✅ Multi-step log viewer modal
✅ Integration patterns documented

### Phase 2: Basic Integration (Next)
- Add LogManager to app Model
- Wire up message types and handlers
- Integrate with ChainStatusModal
- Add keyboard shortcuts
- Basic testing

### Phase 3: Enhanced Features
- Real log fetching via `gh run view --log`
- Export functionality (markdown, HTML, JSON)
- Quick navigation (jump to error, section)
- Pattern detection (timeouts, OOM, etc.)
- Log bookmarks

### Phase 4: Advanced Features
- Timeline view
- Diff mode (compare runs)
- Real-time streaming
- Performance profiling
- Collaboration features

## Quick Start

### For Developers Implementing This:

1. **Review the architecture:**
   ```bash
   cat docs/logs-viewer-integration.md
   ```

2. **Implement Phase 2 integration:**
   - Add `logs.Manager` to `internal/app/app.go`
   - Create message types in `internal/app/messages.go`
   - Add handlers in `internal/app/handlers.go`
   - Update `ChainStatusModal` with 'l' key binding

3. **Test with existing chains:**
   ```bash
   # Run a chain, press 'l' when it completes
   lazydispatch
   ```

4. **Iterate on features:**
   - Start with basic log display
   - Add filtering
   - Enhance with search
   - Implement export

### For Users:

Once implemented, use the log viewer like this:

1. **View logs after chain execution:**
   ```
   # Execute a chain
   lazydispatch > release-bump

   # When complete (success or failure), press 'l'
   # Log viewer opens with all step logs
   ```

2. **Navigate and filter:**
   ```
   tab/← →     Switch between steps
   f           Cycle filter (all/errors/warnings)
   /           Search logs
   n/N         Next/previous match
   ↑↓          Scroll
   q           Close viewer
   ```

3. **View historical logs:**
   ```
   # From history pane
   lazydispatch > History
   # Select entry, press 'l'
   # Logs loaded from cache (if available)
   ```

## Key Innovations

### 1. **Step Separation with Cross-Step Search**
Unlike traditional log viewers that merge everything, this maintains step boundaries while allowing search across all steps. Benefits:
- Easy to identify which step failed
- Context preserved (step name, workflow)
- Can still find patterns across entire execution

### 2. **Error-First Mode**
When opened from a failure, automatically filters to errors. No manual configuration needed.

### 3. **Caching with Intelligence**
- Completed runs: cached for 1 hour
- Active runs: cached for 5 minutes
- Automatic cleanup of expired entries
- Transparent to user

### 4. **Extensible Filter System**
Filter pipeline supports:
- Level filtering (errors, warnings, all)
- Text search (case-sensitive/insensitive)
- Regex patterns
- Step-specific filtering
- Custom filter combinations

### 5. **Match Highlighting**
Search results highlighted in context, making it easy to scan large log files.

## Example User Flows

### Flow 1: Debugging a Failed Deployment

```
User: Runs release-bump chain
→ CI gate passes
→ Version bump fails with permission error

Chain Status Modal appears:
┌──────────────────────────────────────┐
│ Chain: release-bump                  │
│ Status: failed                       │
│                                      │
│ Steps:                               │
│   ✓ ci-gate.yml (completed)         │
│   ✗ version-bump.yml (failed)       │
│                                      │
│ Error Details:                       │
│   Step 2 failed: version-bump.yml   │
│   Error: Permission denied          │
│                                      │
│ [l] view logs  [o] open in browser  │
└──────────────────────────────────────┘

User presses 'l'
→ Logs viewer opens, pre-filtered to errors
→ Shows version-bump.yml tab (failed step)
→ Only error entries visible
→ User sees: "Error: Permission denied writing to file"
→ Suggestion: "Check GitHub token permissions"

User presses 'f' to cycle to 'all' filter
→ Sees full context around error
→ Identifies that write failed during git commit
→ Checks token permissions in repo settings
→ Fixes issue and reruns chain
```

### Flow 2: Investigating Slow Tests

```
User: Notices CI gate step took 8 minutes (usually 3)

User opens logs viewer for successful run
→ Switches to ci-gate.yml tab
→ Presses '/' to search
→ Types "took"
→ Sees highlighted matches:
   - test_api_endpoint took 2.3s ⚠
   - test_db_migration took 4.5s ⚠

User navigates to first slow test
→ Sees: "API health check timed out twice, retried"
→ Context: External API was responding slowly
→ Decision: Increase timeout or mock the API

User presses 'e' to export logs
→ Selects markdown format
→ Exports to ~/Downloads/slow-tests.md
→ Shares with team in Slack
```

### Flow 3: Comparing Runs

```
User: "Why did deployment fail in prod but not staging?"

User opens history
→ Selects staging deployment (succeeded)
→ Presses 'l' to view logs
→ Notes down: "Database migration took 45s"

User opens history again
→ Selects production deployment (failed)
→ Presses 'l' to view logs
→ Sees: "Database migration timed out after 30s"

User presses 'd' (diff mode - future feature)
→ Side-by-side comparison
→ Difference highlighted:
   LEFT (staging):  Migration completed in 45s
   RIGHT (prod):    Migration timeout (30s limit)
→ Solution: Increase prod timeout or optimize migration
```

## Testing Strategy

### Unit Tests
```bash
# Test log structures
go test ./internal/logs -v -run TestLogEntry
go test ./internal/logs -v -run TestFilterConfig

# Test fetcher
go test ./internal/logs -v -run TestFetcher

# Test cache
go test ./internal/logs -v -run TestCache

# Test filter
go test ./internal/logs -v -run TestFilter
```

### Integration Tests
```bash
# Test full log viewer flow
go test ./internal/ui/modal -v -run TestLogsViewerModal

# Test with mock GitHub API
go test ./internal/app -v -run TestLogViewerIntegration
```

### Manual Testing Checklist
- [ ] Log viewer opens when pressing 'l' in ChainStatusModal
- [ ] Tabs show all steps with correct names
- [ ] Error-only filter works when opened from failure
- [ ] Search highlights matches correctly
- [ ] Filter cycling works (all → errors → warnings)
- [ ] Viewport scrolls with arrow keys
- [ ] Window resize updates viewport dimensions
- [ ] Cache loads previously fetched logs
- [ ] Expired cache entries are cleaned up

## Performance Considerations

### Memory Usage
- **Log storage:** ~100-500 KB per step (typical)
- **Cache size:** ~10-50 MB (100 cached runs)
- **Viewport:** Only renders visible lines + buffer

### API Calls
- **Initial load:** 2-3 calls (run + jobs + optionally logs)
- **Cached load:** 0 calls
- **Search/filter:** 0 calls (client-side)

### Optimization Techniques
1. **Lazy loading:** Load step logs only when tab is viewed
2. **Virtual scrolling:** Render only visible portion
3. **Incremental search:** Filter from previous results when possible
4. **Cache with TTL:** Balance freshness vs API usage
5. **Async fetching:** Non-blocking UI during load

## Troubleshooting

### Logs not displaying
**Check:**
- LogManager initialized in app.Model
- GitHub client available
- Network connectivity
- GitHub API rate limits

**Debug:**
```go
log.Printf("LogManager initialized: %v", m.logManager != nil)
log.Printf("Fetching logs for runID: %d", runID)
```

### Search not finding matches
**Check:**
- Case sensitivity setting
- Regex syntax (if enabled)
- Filter level (might be hiding matches)

**Debug:**
```go
log.Printf("Search term: %q", m.filterCfg.SearchTerm)
log.Printf("Case sensitive: %v", m.filterCfg.CaseSensitive)
log.Printf("Matches found: %d", len(matches))
```

### Cache not working
**Check:**
- Cache directory permissions
- Disk space
- Cache TTL configuration

**Debug:**
```go
stats := m.logManager.cache.Stats()
log.Printf("Cache stats: %+v", stats)
```

## Future Enhancements

### Short-term (Next 2-3 releases)
1. Real log fetching via `gh run view --log`
2. Export to markdown/HTML/JSON
3. Quick navigation (jump to error, section)
4. Pattern detection (common errors)
5. Bookmarks for interesting lines

### Medium-term (4-6 releases)
6. Timeline view of execution
7. Diff mode for comparing runs
8. Real-time log streaming
9. Performance profiling extraction
10. Log summarization

### Long-term (Future)
11. AI-powered error analysis
12. Collaboration features (comments, sharing)
13. Integration with external tools (Slack, PagerDuty)
14. Custom log parsers/plugins
15. Multi-repo chain log aggregation

## Contributing

### Adding a New Feature

1. **Design first:**
   - Add to `docs/logs-viewer-features.md`
   - Get feedback from team/users

2. **Implement incrementally:**
   - Start with data structures in `internal/logs/`
   - Add UI components in `internal/ui/modal/`
   - Wire up in app handlers

3. **Test thoroughly:**
   - Unit tests for logic
   - Integration tests for UI
   - Manual testing with real chains

4. **Document:**
   - Update relevant docs
   - Add usage examples
   - Include screenshots if applicable

### Code Style

Follow existing patterns:
- Functional composition over inheritance
- Small, single-responsibility functions
- Error handling at boundaries
- Async operations via channels
- No mutation of shared state

## Support

### Getting Help

1. **Documentation:** Check this guide and linked docs
2. **Code comments:** Review implementation comments
3. **Examples:** Look at `testdata/` for sample configs
4. **Issues:** Report bugs on GitHub
5. **Discussions:** Ask questions in discussions

### Providing Feedback

We'd love to hear:
- Which features you use most
- What's confusing or missing
- Performance issues you encounter
- Ideas for improvements

Open an issue with `[logs-viewer]` prefix.

## License

Same as lazydispatch main project.

---

**Status:** Design complete, ready for implementation
**Last Updated:** 2026-01-19
**Version:** 1.0.0-design
