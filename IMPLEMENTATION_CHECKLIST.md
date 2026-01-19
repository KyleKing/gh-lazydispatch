# Implementation Checklist

Track progress implementing the enhanced chains and log viewer system.

## Phase 0: Setup & Planning

- [x] Review all documentation
- [x] Understand architecture
- [x] Identify integration points
- [ ] Set up development environment
- [ ] Create feature branch

## Phase 1: Commitizen Bump Chain (No Code Changes)

### Workflows

- [ ] Create `.github/workflows/ci-gate.yml`
  - [ ] Implement check status verification
  - [ ] Add required checks filtering
  - [ ] Test with passing CI
  - [ ] Test with failing CI
  - [ ] Test with specific required checks

- [ ] Create `.github/workflows/version-bump.yml`
  - [ ] Set up Python and commitizen
  - [ ] Implement version bumping logic
  - [ ] Add git config and commit
  - [ ] Add push functionality
  - [ ] Test with different bump types

### Chain Configuration

- [ ] Add `release-bump` chain to `.github/lazydispatch.yml`
  - [ ] Define chain variables
  - [ ] Configure ci-gate step
  - [ ] Configure version-bump step
  - [ ] Set appropriate wait conditions
  - [ ] Set failure handling

### Testing

- [ ] Test: CI passes → version bumps
- [ ] Test: CI fails → chain aborts, no bump
- [ ] Test: Specific checks filtering
- [ ] Test: Different bump types (patch, minor, major)
- [ ] Test: Prerelease versions
- [ ] Test: Push vs no-push

## Phase 2: Basic Log Viewer Integration

### Code Setup

- [ ] Copy `internal/logs/*.go` files into project
  - [ ] `types.go`
  - [ ] `fetcher.go`
  - [ ] `gh_fetcher.go`
  - [ ] `cache.go`
  - [ ] `filter.go`
  - [ ] `integration.go`

- [ ] Copy `internal/ui/modal/logs_viewer.go`

- [ ] Run `go mod tidy` to resolve imports

### App Integration

- [ ] Create `internal/app/logs_messages.go`
  - [ ] Define `FetchLogsMsg`
  - [ ] Define `LogsFetchedMsg`
  - [ ] Define `ShowLogsViewerMsg`

- [ ] Update `internal/app/app.go`
  - [ ] Add `logManager *logs.Manager` field to Model
  - [ ] Initialize logManager in NewModel/Init
  - [ ] Set cache directory path
  - [ ] Call `LoadCache()` on startup

- [ ] Update `internal/app/handlers.go` (or app.go Update)
  - [ ] Add `case FetchLogsMsg:`
  - [ ] Add `case LogsFetchedMsg:`
  - [ ] Add `case ShowLogsViewerMsg:`
  - [ ] Implement `fetchLogs()` method
  - [ ] Implement `showLogsViewer()` method
  - [ ] Handle LogsViewerModal updates

- [ ] Update `internal/ui/modal/chain_status.go`
  - [ ] Add `ViewLogs` to keymap
  - [ ] Handle 'l' key press
  - [ ] Send FetchLogsMsg when appropriate
  - [ ] Update help text

### Error Handling Enhancement (Optional for Phase 2)

- [ ] Create `internal/errors/chain_errors.go`
  - [ ] Define `StepExecutionError`
  - [ ] Define `StepDispatchError`
  - [ ] Define `InterpolationError`

- [ ] Update `internal/chain/executor.go`
  - [ ] Use structured errors in runStep()
  - [ ] Include RunURL in errors
  - [ ] Add suggestions to errors

- [ ] Update `internal/ui/modal/chain_status.go`
  - [ ] Add `GetFailedStepRunURL()` method
  - [ ] Add `GetDetailedError()` method
  - [ ] Update View() to show URLs
  - [ ] Add 'o' key to open browser

- [ ] Add error styles to `internal/ui/styles.go`
  - [ ] `ErrorTitleStyle`
  - [ ] `ErrorStyle`
  - [ ] `URLStyle`

### Build & Test

- [ ] Build successfully: `go build`
- [ ] Run: `./lazydispatch`
- [ ] Test: Execute chain, press 'l' when complete
- [ ] Verify: Modal opens with logs
- [ ] Verify: Can switch tabs
- [ ] Verify: Can search
- [ ] Verify: Can filter

## Phase 3: Real Log Fetching

### gh CLI Integration

- [ ] Verify gh CLI available: `gh --version`
- [ ] Test gh auth: `gh auth status`
- [ ] Test log fetch: `gh run view <run-id> --log`

### Code Updates

- [ ] Update `internal/logs/integration.go`
  - [ ] Use `NewGHFetcher` instead of `NewFetcher`
  - [ ] Check gh CLI availability
  - [ ] Handle gh CLI errors gracefully

- [ ] Test with real workflow runs
  - [ ] Verify logs parse correctly
  - [ ] Check step boundaries detected
  - [ ] Validate error detection works

### Error Handling

- [ ] Handle gh CLI not installed
- [ ] Handle gh CLI not authenticated
- [ ] Handle network errors
- [ ] Handle malformed log output

### Testing

- [ ] Test: Fetch logs from successful run
- [ ] Test: Fetch logs from failed run
- [ ] Test: Parse step boundaries correctly
- [ ] Test: Detect errors accurately
- [ ] Test: Handle large logs (>10k lines)

## Phase 4: Enhanced Filtering & Search

### Match Navigation

- [ ] Implement `jumpToNextMatch()`
- [ ] Implement `jumpToPrevMatch()`
- [ ] Add visual indicator of current match
- [ ] Show match count (e.g., "3 of 15")

### Quick Filters

- [ ] Add preset filters to keymap
  - [ ] 'e' = errors only
  - [ ] 'w' = warnings and errors
  - [ ] 'a' = all logs

- [ ] Add filter indicator to status line

### Case Sensitivity Toggle

- [ ] Add 'i' key to toggle case sensitivity
- [ ] Update search indicator

### Testing

- [ ] Test: Navigate matches with n/N
- [ ] Test: Quick filters work correctly
- [ ] Test: Case sensitivity affects results
- [ ] Test: Search performance with large logs

## Phase 5: Export Functionality

### Markdown Export

- [ ] Implement `exportMarkdown()` in logs package
- [ ] Add export modal or inline prompt
- [ ] Handle file path selection
- [ ] Include filter state in export
- [ ] Add timestamp and metadata

### UI Integration

- [ ] Add 'x' or 'e' key for export
- [ ] Show export success/failure message
- [ ] Allow clipboard copy of export

### Testing

- [ ] Test: Export all logs
- [ ] Test: Export filtered logs
- [ ] Test: Export search results
- [ ] Test: Open exported file
- [ ] Test: Share exported file

## Phase 6: History Integration

### History Entry Updates

- [ ] Update `internal/frecency/types.go`
  - [ ] Add `StepResults []HistoryStepResult`
  - [ ] Define `HistoryStepResult` struct

- [ ] Update chain execution handler
  - [ ] Store step results in history
  - [ ] Convert `chain.StepResult` to `frecency.HistoryStepResult`

### History Pane Integration

- [ ] Add 'l' key to history pane
- [ ] Implement `reconstructChainStateFromHistory()`
- [ ] Fetch logs for historical entry
- [ ] Load from cache if available

### Testing

- [ ] Test: View logs from history
- [ ] Test: Cache hit for recent execution
- [ ] Test: Cache miss for old execution
- [ ] Test: Reconstruct chain state correctly

## Phase 7: Advanced Features (Optional)

### Timeline View

- [ ] Design timeline UI
- [ ] Extract timestamps from logs
- [ ] Calculate step durations
- [ ] Render timeline visualization
- [ ] Add 't' key to toggle view

### Pattern Detection

- [ ] Define common patterns
  - [ ] Timeout patterns
  - [ ] Memory errors
  - [ ] Network errors
  - [ ] Permission errors

- [ ] Implement pattern matching
- [ ] Show detected patterns in UI
- [ ] Allow jump to pattern occurrences

### Log Bookmarks

- [ ] Implement bookmark storage
- [ ] Add 'm' key to mark line
- [ ] Add 'M' key to view bookmarks
- [ ] Persist bookmarks to disk

### Diff Mode

- [ ] Implement side-by-side layout
- [ ] Load two runs for comparison
- [ ] Highlight differences
- [ ] Add alignment scrolling

### Log Streaming

- [ ] Implement polling for active runs
- [ ] Update logs incrementally
- [ ] Add auto-scroll option
- [ ] Show "live" indicator

## Testing & Quality Assurance

### Unit Tests

- [ ] Test log types and structures
- [ ] Test filter logic
- [ ] Test cache operations
- [ ] Test search matching
- [ ] Test export functionality

### Integration Tests

- [ ] Test full log fetch flow
- [ ] Test modal lifecycle
- [ ] Test with real GitHub API
- [ ] Test error handling

### Manual Testing

- [ ] Test on macOS
- [ ] Test on Linux
- [ ] Test with slow network
- [ ] Test with large logs (>50k lines)
- [ ] Test with unicode characters
- [ ] Test with ANSI color codes

### Performance Testing

- [ ] Measure load time for 10k line log
- [ ] Measure search time
- [ ] Measure filter time
- [ ] Check memory usage
- [ ] Profile with `go test -bench`

## Documentation Updates

### User Documentation

- [ ] Add log viewer section to README
- [ ] Add screenshots of log viewer
- [ ] Document keyboard shortcuts
- [ ] Add troubleshooting section

### Developer Documentation

- [ ] Document log system architecture
- [ ] Add code examples
- [ ] Document extension points
- [ ] Update CONTRIBUTING guide

## Release Preparation

### Code Review

- [ ] Self-review all changes
- [ ] Run linters: `mise run ci`
- [ ] Fix any issues
- [ ] Ensure tests pass

### Git Operations

- [ ] Create feature branch
- [ ] Commit incrementally with good messages
- [ ] Push branch
- [ ] Create pull request

### PR Checklist

- [ ] PR description links to design docs
- [ ] Screenshots included
- [ ] Tests pass in CI
- [ ] Code reviewed by maintainer
- [ ] Documentation reviewed

## Post-Implementation

### Monitor

- [ ] Watch for bug reports
- [ ] Monitor performance in production
- [ ] Collect user feedback

### Iterate

- [ ] Address feedback
- [ ] Fix bugs
- [ ] Add requested features
- [ ] Optimize performance

---

## Progress Summary

**Phase 0:** ⬜ Not started
**Phase 1:** ⬜ Not started (Commitizen chain)
**Phase 2:** ⬜ Not started (Basic integration)
**Phase 3:** ⬜ Not started (Real logs)
**Phase 4:** ⬜ Not started (Enhanced features)
**Phase 5:** ⬜ Not started (Export)
**Phase 6:** ⬜ Not started (History)
**Phase 7:** ⬜ Not started (Advanced)

**Overall:** 0% complete

---

## Notes

Track blockers, decisions, and deviations from the plan here:

- [ ] Decision: Use synthetic logs initially, real logs in Phase 3
- [ ] Decision: Error enhancement optional for Phase 2
- [ ] Decision: Start with markdown export, add others later

---

**Last Updated:** 2026-01-19
**Current Phase:** Phase 0 (Planning)
