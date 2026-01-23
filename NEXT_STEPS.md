# Release Checklist

Pre-release tasks for gh-lazydispatch v1.0.

## Critical (Blocking Release)

- [x] Fix failing tests in `internal/logs/`:
  - [x] `TestLogStreamer_PollingBehavior` - fixed: always send status update on poll
  - [x] `TestIntegration_ANSIColorCodes` - fixed: added `##[group]` markers to test fixture
- [ ] Run `mise run ci` with all tests passing

## High Priority

- [x] Implement error modal for log fetch failures (`internal/ui/modal/error.go`)
- [x] Add log viewer documentation to README.md:
  - [x] Document `l` key in Chain Status modal
  - [x] Document `l` key in History pane
  - [x] Add keyboard shortcut table for log viewer
- [x] Increase test coverage for UI packages:
  - [x] `internal/ui/modal` (15.7% -> 26.0%)
  - [x] `internal/ui/panes` (24.4% -> 57.6%)

## Medium Priority

- [x] Review and consolidate `docs/` - merged test-safety into CONTRIBUTING.md, kept chain-examples.md
- [x] Add integration test for full chain execution with log viewing
- [ ] Benchmark log filtering with large logs (>10k lines)

## Low Priority (Post-Release)

- [ ] Implement export functionality (markdown export for logs)
- [ ] Add timeline view for log visualization
- [ ] Pattern detection for common errors (timeouts, OOM, permissions)

## Documentation Cleanup

Removed outdated development/design docs (implementation complete):

- [x] `IMPLEMENTATION_CHECKLIST.md`
- [x] `IMPLEMENTATION_SUMMARY.md`
- [x] `docs/logs-viewer-quickstart.md`
- [x] `docs/implementation-guide.md`
- [x] `docs/logs-viewer-integration.md`
- [x] `docs/logs-viewer-features.md`
- [x] `docs/README-logs.md`
- [x] `docs/chain-failure-alerting.md`

Kept user/developer-facing docs:

- `README.md` - user documentation (updated with log viewer)
- `CONTRIBUTING.md` - developer guide with architecture and test safety
- `AGENTS.md` / `CLAUDE.md` - AI assistant guidelines
- `TESTING.md` - test strategy
- `UX.md` - UI layout documentation
- `docs/chain-examples.md` - advanced chain configuration examples
- `testdata/README.md` - test fixtures explanation

## Verification

```bash
# Run full CI
mise run ci

# Check test coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total

# Verify no regressions
go test -race ./...
```
