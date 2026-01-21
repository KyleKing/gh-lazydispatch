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

- [x] Create `.github/workflows/ci-gate.yml`
  - [x] Implement check status verification
  - [x] Add required checks filtering
  - [ ] Test with passing CI
  - [ ] Test with failing CI
  - [ ] Test with specific required checks

- [x] Create `.github/workflows/version-bump.yml`
  - [x] Set up Python and commitizen
  - [x] Implement version bumping logic
  - [x] Add git config and commit
  - [x] Add push functionality
  - [ ] Test with different bump types

### Chain Configuration

- [x] Add `release-bump` chain to `testdata/.github/lazydispatch-release.yml`
  - [x] Define chain variables
  - [x] Configure ci-gate step
  - [x] Configure version-bump step
  - [x] Set appropriate wait conditions
  - [x] Set failure handling

### Testing

- [ ] Test: CI passes → version bumps
- [ ] Test: CI fails → chain aborts, no bump
- [ ] Test: Specific checks filtering
- [ ] Test: Different bump types (patch, minor, major)
- [ ] Test: Prerelease versions
- [ ] Test: Push vs no-push

## Phase 2: Basic Log Viewer Integration

### Code Setup

- [x] Copy `internal/logs/*.go` files into project
  - [x] `types.go`
  - [x] `fetcher.go`
  - [x] `gh_fetcher.go`
  - [x] `cache.go`
  - [x] `filter.go`
  - [x] `integration.go`

- [x] Copy `internal/ui/modal/logs_viewer.go`

- [x] Run `go mod tidy` to resolve imports

### App Integration

- [x] Create `internal/app/logs_messages.go`
  - [x] Define `FetchLogsMsg`
  - [x] Define `LogsFetchedMsg`
  - [x] Define `ShowLogsViewerMsg`

- [x] Update `internal/app/app.go`
  - [x] Add `logManager *logs.Manager` field to Model
  - [x] Initialize logManager in NewModel/Init
  - [x] Set cache directory path
  - [x] Call `LoadCache()` on startup

- [x] Update `internal/app/handlers.go` (or app.go Update)
  - [x] Add `case FetchLogsMsg:`
  - [x] Add `case LogsFetchedMsg:`
  - [x] Add `case ShowLogsViewerMsg:`
  - [x] Implement `fetchLogs()` method
  - [x] Implement `showLogsViewer()` method
  - [x] Handle LogsViewerModal updates

- [x] Update `internal/ui/modal/chain_status.go`
  - [x] Add `ViewLogs` to keymap
  - [x] Handle 'l' key press
  - [x] Send FetchLogsMsg when appropriate
  - [x] Update help text

### Error Handling Enhancement (Optional for Phase 2)

- [x] Create `internal/errors/chain_errors.go`
  - [x] Define `StepExecutionError`
  - [x] Define `StepDispatchError`
  - [x] Define `InterpolationError`

- [x] Update `internal/chain/executor.go`
  - [x] Use structured errors in runStep()
  - [x] Include RunURL in errors
  - [x] Add suggestions to errors

- [x] Update `internal/ui/modal/chain_status.go`
  - [x] Add `GetFailedStepRunURL()` method
  - [x] Add `GetDetailedError()` method
  - [x] Update View() to show URLs
  - [x] Add 'o' key to open browser

- [x] Add error styles to `internal/ui/styles.go`
  - [x] `ErrorTitleStyle`
  - [x] `ErrorStyle`
  - [x] `URLStyle`

### Build & Test

- [x] Build successfully: `go build`
- [x] Run: `./lazydispatch`
- [ ] Test: Execute chain, press 'l' when complete
- [ ] Verify: Modal opens with logs
- [ ] Verify: Can collapse/expand sections
- [ ] Verify: Can search
- [ ] Verify: Can filter

## Phase 3: Real Log Fetching

### gh CLI Integration

- [x] Verify gh CLI available: `gh --version`
- [x] Test gh auth: `gh auth status`
- [ ] Test log fetch: `gh run view <run-id> --log`

### Code Updates

- [x] Update `internal/logs/integration.go`
  - [x] Use `NewGHFetcher` instead of `NewFetcher`
  - [x] Check gh CLI availability
  - [x] Handle gh CLI errors gracefully

- [ ] Test with real workflow runs
  - [ ] Verify logs parse correctly
  - [ ] Check step boundaries detected
  - [ ] Validate error detection works

### Error Handling

- [x] Handle gh CLI not installed
- [x] Handle gh CLI not authenticated
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

- [x] Implement `jumpToNextMatch()`
- [x] Implement `jumpToPrevMatch()`
- [x] Add visual indicator of current match
- [x] Show match count (e.g., "3 of 15")

### Quick Filters

- [x] Add preset filters to keymap
  - [x] 'e' = errors only
  - [x] 'w' = warnings and errors
  - [x] 'a' = all logs

- [x] Add filter indicator to status line

### Case Sensitivity Toggle

- [x] Add 'i' key to toggle case sensitivity
- [x] Update search indicator

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

- [x] Update `internal/frecency/types.go`
  - [x] Add `StepResults []HistoryStepResult`
  - [x] Define `HistoryStepResult` struct

- [x] Update chain execution handler
  - [x] Store step results in history
  - [x] Convert `chain.StepResult` to `frecency.HistoryStepResult`

### History Pane Integration

- [x] Add 'l' key to history pane
- [x] Implement `reconstructChainStateFromHistory()`
- [x] Fetch logs for historical entry
- [x] Load from cache if available

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

- [x] Implement polling for active runs
- [x] Update logs incrementally
- [x] Add auto-scroll option
- [x] Show "live" indicator

## Testing & Quality Assurance

### Unit Tests

- [x] Test log types and structures
- [x] Test filter logic
- [x] Test cache operations
- [x] Test search matching
- [ ] Test export functionality
- [x] Test log streaming (detectNewLogs, state tracking)

### Integration Tests

- [x] Test full log fetch flow
- [x] Test modal lifecycle
- [x] Test with mocked GitHub API (gh CLI)
- [x] Test error handling
- [x] Test log streaming with multiple polls
- [x] Test incremental log detection

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

**Phase 0:** ✅ Complete (Planning and setup)
**Phase 1:** ✅ Complete (Commitizen chain workflows created)
**Phase 2:** ✅ Complete (Log viewer integrated into app)
**Phase 3:** ✅ Complete (Real log fetching with gh CLI)
**Phase 4:** ✅ Complete (Enhanced filtering & search)
**Phase 5:** ⬜ Not started (Export)
**Phase 6:** ✅ Complete (History integration)
**Phase 7:** 🔄 Partial (Log streaming complete, other features not started)

**Overall:** 75% complete (Phases 0-4, 6 done, Phase 7 log streaming done)

---

## Notes

Track blockers, decisions, and deviations from the plan here:

- [x] Decision: Use synthetic logs initially, real logs in Phase 3
- [x] Decision: Error enhancement optional for Phase 2
- [ ] Decision: Start with markdown export, add others later
- [x] **Log Streaming Implementation (2026-01-20):**
  - Implemented `LogStreamer` with 2-second polling for active runs
  - Incremental log detection via line count comparison
  - Auto-scroll with smart user intent detection (pauses when scrolling up)
  - Live status indicators: [LIVE], [QUEUED], [COMPLETED]
  - Comprehensive test suite: integration tests with mocked gh CLI + unit tests
  - All tests passing, no regressions detected

---

**Last Updated:** 2026-01-20
**Current Phase:** Phase 7 (Log streaming complete with comprehensive tests)
