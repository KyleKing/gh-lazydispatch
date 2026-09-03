## v1.12.0 (2026-09-03)

### Feat

- **cli**: add gh lazydispatch watch, the pr-merge-watch replacement
- **chain**: adopt an already-running run with source: existing

### Fix

- **chain**: close races and gaps in source: existing adoption

## v1.11.1 (2026-09-03)

### Fix

- **diagnose**: Report no failed steps for a run that succeeded

## v1.11.0 (2026-09-02)

### Feat

- **git**: resolve the checkout root holding a working directory

### Fix

- **ui**: paint the modal ground behind every cell
- read workflows and chains from the checkout root, not the working directory
- **app**: show why a chain config could not be read instead of no chains
- **config**: name the broken symlink instead of reporting a missing file

### Refactor

- **config**: split the load path so each helper states one failure

## v1.10.0 (2026-09-02)

### Feat

- **timeline**: open a step's own log from the step
- **runs**: re-run and cancel a run from the row that names it
- **dispatch**: reject locally what the dispatch would fail on
- **inputs**: resolve environment inputs against the repository
- **ui**: rebuild the layout around what each thing actually is

### Fix

- **exec**: let the test guard tell a read from a write per operation
- **remap**: clear the options the wizard already decided
- **ui**: overlay an input's details where the pane cannot hold them

### Refactor

- **modal**: center overlays through aragonite

## v1.9.0 (2026-09-01)

### Feat

- **runs**: answer a pull request scope with its own check rollup

### Refactor

- **ui**: render relative times through aragonite's display package
- **logs**: cache a run's log where the cache is actually read
- **git**: read branches through aragonite's vcs layer
- **github**: take the Actions model from aragonite rather than converting it
- **testutil**: keep the command mock with the other test doubles

## v1.8.0 (2026-09-01)

### Feat

- **cli**: answer whether a branch is green with export runs --current

## v1.7.0 (2026-09-01)

### Feat

- **ui**: show a branch's real state and give the layout its space back

## v1.6.0 (2026-09-01)

### Feat

- **github**: read Actions runs and jobs through aragonite
- **app**: read any run's log from the command bar

### Fix

- **logs**: read a failure signature only from the failure it describes
- **ui**: scroll the workflow list instead of overflowing the pane

## v1.5.0 (2026-09-01)

### Feat

- **timeline**: draw a run's jobs on one time axis

## v1.4.0 (2026-09-01)

### Feat

- **app**: add an action leader and a command bar

## v1.3.1 (2026-09-01)

### Refactor

- use aragonite's ghcassette instead of a local copy

## v1.3.0 (2026-09-01)

### Feat

- **cli**: add export commands for reading runs, logs, and failures

### Fix

- **logs**: stop reading a step's own script as its failure, and parse jobs gh cannot map to steps

### Refactor

- **panes**: delete the unreachable ConfigModel duplicate

## v1.2.2 (2026-09-01)

### Fix

- **logs**: parse the log format gh actually emits, and let async updates through

## v1.2.1 (2026-09-01)

### Fix

- **chain**: resolve a step's previous against the step above it, in one place

## v1.2.0 (2026-09-01)

### Feat

- **logs**: export a run's logs as markdown with its failure signatures named

### Fix

- **ui**: stop reserving pane borders twice, which cut every pane's content by two
- **ui**: make focus, defaults, and column widths readable without color
- **workflow**: report unparseable workflow files instead of showing an empty repo
- **keys**: stop the live-view and input bindings shadowing tab, digit, and enter keys

### Refactor

- **ui**: take the palette and table fitting from aragonite

## v1.1.1 (2026-08-31)

### Fix

- **scripts**: generate the tap deploy key inside 1Password

## v1.1.0 (2026-07-30)

### Feat

- **release**: publish a Homebrew cask from goreleaser

## v1.0.5 (2026-07-30)

### Fix

- **release**: build each target into its own dist path
- quote workflow run steps to avoid unparseable YAML in test fixtures
- quote workflow env values and drop empty choice option for actionlint

## v1.0.4 (2026-07-26)

### Fix

- **lint**: add missing t.Parallel calls in the linux-only test file

## v1.0.3 (2026-07-26)

### Fix

- **lint**: resolve golangci-lint 2.12.2 findings surfaced by the template update
- guard the workflow index in the input details pane

## v1.0.2 (2026-07-05)

### Fix

- finish v2 linter migration

## v1.0.1 (2026-07-04)

### Refactor

- fix gosec, complexity, duplication, and remaining misc lint findings
- allow BubbleTea Model and modal.Context interfaces in ireturn
- fix err113, errcheck, wrapcheck, and nilnil lint findings
- fix gocritic, govet, revive, and nonamedreturns lint findings

## v1.0.0 (2026-07-04)

### Feat

- migrate to my template
- add error modal
- expand test coverage and integration
- implement log streaming
- add history integration
- add additional error handling
- replace go-gh with go-cli
- implement real log fetching
- exploring adding workflow logs to the TUI
- major improvements to chain capabilities and usefulness
- add context-aware key hints and tabulated history
- redesign UX to better integrate new chains and history
- add chaining, history, and rules
- handle invalid history entries
- add history detail pane, more selective dimming, and faster selection
- improve layout and interaction edge cases
- add filter-selection to the branches
- rename to wfd for consistent terminology with Actions
- rename to gh-wfr
- add catppuccin themes
- integrate modal system and execution flow
- Phase 9 - polish with help modal and CLI flags
- Phase 8 - execution with command display
- Phase 7 - modal system with lazygit-style stack
- Phases 4-6 - workflow, history, and config panes
- Phase 3 - TUI skeleton with 3-pane layout
- Phase 2 - frecency store with XDG cache
- Phase 1 - workflow discovery and YAML parsing

### Fix

- resolve goconst and lll lint findings
- resolve goconst and lll lint findings
- resolve mechanical lint findings (misspell, godot, nlreturn, etc.)
- drop mise-managed gsa pin, resolve dual-bump-workflow question
- resolve testpackage, unused, unparam, usetesting, and thelper findings
- adopt unreleased template fixes for mise tasks and golangci v2
- resolve issues with releaser
- golangci-lint config format and linting errors (#1)
- migrate to v2 of the golangci configuration
- don't use real gh-cli when testing
- use <C-r> for restore and minor visual patches
- finish lazydispatch rename with gh- prefix
- correctly handle the modal

### Refactor

- extract repeated string literals into constants (goconst)
- replace magic numbers with named constants (mnd)
- switch tests to internal packages and fix errcheck findings
- run v2 golangci and fix test mocking
- add workflows for testing
- improve error handling, tests, and cohesion
- DRY long conditionals
- minor defaults and command cleanup


- rename once more to lazydispatch
