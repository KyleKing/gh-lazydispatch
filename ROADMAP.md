# Roadmap

Phased plan for gh-lazydispatch after the v1.0.0 release (2026-07-04). Open defects and immediate follow-ups live in `NEXT_STEPS.md`.

Phase 0 (housekeeping: the golangci v2 backlog, the gsa build failure, the dual bump-workflow risk, and the copier adoption) is complete and is not tracked here.

Phases 1 (markdown log export) and 2 (error pattern detection) shipped together,
because the export's "Detected issues" section is the detector's first consumer.
`internal/logs/export.go` holds `ExportAsMarkdown` and `internal/logs/patterns.go`
holds the signature set; `x` in the log viewer writes the file.

## Phase 3: Timeline view (shipped)

`internal/ui/timeline` lays spans on a shared axis and holds no terminal
vocabulary; `internal/ui/panes/timeline.go` renders it as the fourth
right-panel tab, with `enter` drilling from jobs into one job's steps. Filled
by `a` then `t`, or by `:timeline <run-id>`.

## Phase 4: keyboard composability (shipped)

`a` opens the verbs scoped to whatever has focus, and `:` opens a command bar
with tab completion. Ported from
[gh-repo-dashboard](https://github.com/KyleKing/gh-repo-dashboard), which keeps
the two as separate grammars on purpose: a menu is a list you read, a bar is a
name you type.

`:logs <run-id>` and `:diagnose <run-id>` were added later, because a run this
checkout never dispatched had no route into the TUI at all.

## What driving a 26-workflow repository found

Everything below Phase 5 came out of running the TUI under a PTY against a
repository with 51 workflows, 26 of them dispatchable, and diagnosing 42 of its
real runs. Two defects that every fixture in this repository had agreed were
fine:

- The workflow pane drew one row per workflow against a minimum-height style, so
  the layout outgrew the terminal and the footer, the command bar, and the config
  panel's bottom border fell off the screen. `TestViewAtSizes` passed because its
  fixture repository has two workflows
- 10 of 12 signature matches were false, and `errors` kept a long step's first
  twenty error lines rather than its last

Both are fixed. The lesson is in `AGENTS.local.md`: the demo workflows prove the
code runs, and only a repository whose logs nobody wrote for this tool proves it
is right. The same applies to the frames: the golden tests rendered against
zero-value styles, so they had been asserting on a UI with no borders and no
colors that no user has ever seen.

## Phase 5: the branch's current state, not this checkout's history (shipped)

The History tab lists local dispatch history, so in a repository this tool has
never dispatched from it read "No recent runs" while GitHub held thousands. The
question a reader opens the tool with is whether their branch is green, and
nothing on screen answered it.

`internal/ui/panes/runs.go` is the Runs tab over
`aragonite/forge/github.LatestRunsOnBranch`: the newest run of each workflow on a
ref, keyed on workflow file *and* display title so a workflow reporting its mode
in the title keeps one current state per mode. It loads on first open, `s` cycles
branch / my PRs / awaiting my review, and the status bar carries the verdict.

The pull request scopes list one row per pull request with its own check rollup,
from `SearchPRsInRepo`: one call and exact, where grouping a page of the
repository's recent runs by head branch reported nothing for every pull request
but the last one to run. The pane holds two row shapes, and `enter` on a rollup
drills into the runs on that pull request's head branch, which is what names the
failing workflow.

Still to build:

- A flakiness view. Per-branch, one run per workflow is the whole answer; asking
  whether a workflow is flaky is the opposite query (many runs of one workflow)
  and wants its own screen rather than a column

## Phase 6: layout from first principles (shipped)

`internal/app/layout.go` is now the only place the split is decided, and both
`View` and the resize handler take it from there. The right panel was previously
told it had one more row than it rendered in.

What the redesign settled:

- The config pane takes the height its content needs, between its own chrome and
  leaving `minTopPaneHeight` rows above it. A workflow with no inputs no longer
  spends half an 80x24 terminal on an empty table: the workflow list went from
  11 rows to 26 in a repository with 26 dispatchable workflows
- The tab bar carries each tab's count, and the Runs tab its verdict, so what is
  behind `h`/`l` is readable without going there. Names abbreviate to initials
  before a count is dropped
- Every list scrolls through `ui.ScrollWindow`. History and Live drew every row
  and relied on `MaxHeight` to cut the overflow, which silently hid the selection
- The config pane's row budget was `height - 14` against a pane 11 rows of chrome
  tall, so on a workflow with many inputs the command preview was the part that
  got truncated

Two defects the golden frames could not have caught before, because the frames
themselves were wrong: `PaneStyle` called `BorderStyle(...)`, which sets a
border's style without enabling any side, so no pane had ever drawn a border and
focus was invisible. Nothing outside `main` called `ui.InitTheme`, so every test
rendered against zero-value styles. `internal/ui` now applies a default palette
at init, which is what makes a golden frame the frame a user sees.

Golden frames at 80x24, 120x40, and 160x50 hold the layout, alongside
`assertFits` and `TestLayoutFor_GivesTheConfigPaneWhatItsContentNeeds`.

## Phase 7: aragonite consolidation

`tui/theme`, `tui/table`, and `ghcassette` were already shared.
`aragonite/forge/github` now holds the Actions reads (run by ID, run list by
query, jobs with per-step timings, latest run per workflow on a ref) and exports
`WithRunner`, which is what lets this repository keep `internal/exec`'s mutation
guard in front of every gh call rather than reinstating it per call site.

`internal/github`'s read paths delegate through that seam, converting aragonite's
model at the boundary. `LatestPerWorkflow` is exported separately from
`LatestRunsOnBranch` because the pull request scopes group a listing this
repository fetched rather than one aragonite fetched for a single ref.

Remaining, in the order that pays off:

- Alias `github.WorkflowRun`, `Job`, and `Step` to their `forge` counterparts so
  the model is aragonite's rather than a conversion of it. About 20 files
  reference them, and `HTMLURL` versus `URL` plus the local `IsSuccess` are the
  only real edits
- `internal/git` to `aragonite/vcs`, which brings jj colocated checkouts along
  for free
- `internal/logs/cache.go` to `aragonite/cache`
- `formatTimeAgo` in `internal/ui/panes/history.go` to `aragonite/display`, which
  already spells relative times and status glyphs for two other tools

## Coverage

`mise run test:coverage-min` holds the whole module at 70%. Two packages sit
below it for different reasons:

- `internal/chain` was at 45% because the engine dispatched through
  `runner.ExecuteAndGetRunID`, which shells out to `gh workflow run`, and
  `internal/exec`'s mutation guard panics on that in a test. The executor now
  takes `WithDispatcher` and `WithPollInterval`, which took it to 78%. Every
  seam like that is worth adding: it is a design defect that reads as a coverage
  number
- `internal/exec` reported 23.8% while every uncovered statement was
  `mock_executor.go`, which other packages' tests exercise heavily and its own
  never touch. The mock now lives in `internal/testutil` alongside the rest of
  the test doubles, and `exec` reports 95.2% of the code that actually ships

## Deferred / v2 ideas

Tracked in `DESIGN.md` and `CONTRIBUTING.md` design-decision sections:

- Marks and operator-object batch verbs (`!d` over a marked set) — the palette
  won over them. `space` is bound to focus-config, so adopting them means moving
  that first
- Live timeline redraw. The layout already closes an open span at `now`; what is
  missing is a redraw tick, not arithmetic
- Single-screen dashboard alternative to the modal-stack UX
- SQLite-backed frecency store (currently JSON)
- `environment`-type input resolution via repo-environments API call
- Ranking failure signatures rather than reporting each once per step. Scoping the
  scan to a failed step's last 200 lines removed the false positives that made
  this look urgent, so it stays deferred until a real corpus shows noise again
