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

`internal/github`'s read paths delegate through that seam. `LatestPerWorkflow` is
exported separately from `LatestRunsOnBranch` because the pull request scopes
group a listing this repository fetched rather than one aragonite fetched for a
single ref.

`github.WorkflowRun`, `Job`, and `Step` are aliases of their `forge`
counterparts, so the model is aragonite's rather than a conversion of it. That
turned up a defect the conversion had been hiding: `GetLatestRun` filtered on
`actions/runs?workflow=<name>`, which the runs collection has no such parameter
for, so it answered with the repository's newest run whatever workflow was
asked for.

`internal/git` reads through `aragonite/vcs`, which brings jj checkouts along for
free. The branch list and the default branch are `vcs.RemoteBranches` and
`vcs.DefaultBranchName`, and what is left here is the timeout and the fallback
list.

A run's logs are held in `aragonite/cache`'s TTL cache, which replaced 200 lines
of hand-rolled disk cache that nothing ever wrote to: `Put` had no caller, so
every reopen of a run re-downloaded its log. Memory only, because aragonite's
disk store deliberately holds counts, states, and titles rather than bodies, and
a log is measured in megabytes.

Relative times come from `aragonite/display`. `RelativeTime` was too wide for a
table column measured in single digits of cells, so the compact form
(`RelativeTimeCompact`) went upstream rather than staying a local copy.

Phase 7 is complete. What is left in `internal/github` is this tool's own
vocabulary: the status and conclusion constants, the dispatch path, and the
mutation guard the reads travel through.

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

## Phase 8: the layout lazygit already settled

Driving the TUI beside lazygit turned up a set of defects that share one cause:
the layout named things by where they were built rather than by what they are.

- The right panel held History, Chains, Live, Timeline, and Runs as peers, and
  they are not peers. Chains are dispatch *targets*, like the workflows they are
  built from, so they belong in the left column beside them. A timeline is one
  run's detail, so it belongs behind the row that names the run
- The Timeline tab was reachable by `a` then `t`, offered only from the History
  and Live tabs. The default focus is the workflow list, whose menu offers no
  such verb, so the documented route did nothing from the screen the tool opens
  on. It is now `enter` on a Runs, Live, or Flaky row, with a breadcrumb naming
  the list it came from and `esc` peeling one layer per press
- `h`/`l` cycled tabs, which is what left them unavailable for the move a
  two-column layout is actually asked for. `[`/`]` cycle tabs (from anywhere:
  the right panel is the only tabbed thing on screen) and `h`/`l` cross between
  the columns
- The active tab was drawn as `[History]`. Every other bracket in this UI is a
  key you can press, and one that is not reads as a broken hint. The active tab
  is a filled segment
- The same counts were printed three times: the status bar, the tab bar, and the
  rows. The status bar now carries only what no pane reports, which is the ref
  every dispatch targets and a running chain's progress
- The left column was 11/30 of the terminal against workflow names half that
  wide, and drew 26 empty rows under seven workflows. It is 30% between 24 and
  40 cells, stacking Workflows, Chains, and Config, with the right panel running
  the full height
- `executionDoneMsg` had no handler at all, so a dispatch that failed looked
  exactly like one that worked. It reports the failure, and on success moves the
  right panel to the list that now has something to say

Three defects the golden frames could not have caught, all found by rendering
under a PTY at 80, 120, and 160 cells:

- A pane whose header wrapped one line too far lost its bottom border to
  `MaxHeight` rather than overflowing, so `assertFits` passed on a frame with a
  missing border. `assertBottomPaneCloses` holds that now
- Dropping a table column by priority is not enough on its own in a narrow pane:
  the header then carries an overflow marker naming what it dropped, and that
  marker is what wraps. `ui.ConfigColumnsFor` picks a column set that fits
- `RunsModel.View` carried the scope in its pane title, and the tabbed panel
  renders only `ViewContent`. The ref a pull request row drilled into had never
  been on screen

Also in this phase, from the backlog:

- **Flakiness view.** One page of the repository's runs, grouped by workflow and
  sorted flakiest first, and narrowing to the workflow selected on the left
  without a second call. A workflow with nothing finished has no rate rather
  than a zero one, and sorts after every measured one
- **Live timeline redraw.** A tick while any bar on screen is still open, which
  stops as soon as they all have an end
- **Marks and operator-object verbs.** `space` marks a row in the workflow list
  or the Live tab, and a verb acts on the set: `enter` confirms every dispatch at
  once, `d` stops watching every marked run. `space` was focus-config, which
  `tab` already does

## Phase 9: acting on what the tool already reads

The tool could read a failed run, lay it on a time axis, and name what broke
it, and then the only way to act on any of that was the browser.

- **Re-run and cancel.** `gh run rerun --failed` and `gh run cancel` from the
  action menu wherever a run is selected: Runs, Live, Flaky, and an open
  timeline. Both are confirmed the way a dispatch is, since re-running or
  canceling somebody's run is no less outward-facing than starting one, and the
  confirmation shows the gh command rather than describing it
- **A step's own log.** The timeline names the step that took the time or
  failed, and reading it meant opening the whole run's log and finding that
  step again. The action menu opens it on that step, with the rest of the run
  folded around it rather than hidden
- **Environment inputs.** An `environment`-type input names one of the
  repository's deployment environments, which nothing in the workflow file
  lists, so the modal had been a free-text box with an apology in it. The names
  come from `repos/{owner}/{repo}/environments`, read once at startup and only
  where a workflow declares such an input
- **Pre-dispatch validation.** A choice set outside its options and a boolean
  carrying "yes" went out and came back a 422 naming nothing.
  `validation.ValidateDispatch` names the input instead

Also in this phase, `internal/ui` got its first test file. It holds `PaneBox`,
`ScrollWindow`, and the column sets, which is where the last three rendering
defects lived, and it had 0% coverage. The `PaneBox` test holds the box to its
rectangle for any content, and the column test found that `ConfigColumnsFor`
took a table width while reading as a pane width.

## Phase 10: testing what only a recording can test

Coverage went 75.0% to 85.6%, and the ten points came from deleting code as
much as from writing tests. `internal/testutil/testutil.go` was 208 lines of
assert helpers with no callers, and `internal/logs/test_helpers.go` was
benchmark-only scaffolding in a non-test file, so its statements counted
against the denominator while never running under `go test`.

The tests that followed each sweep one axis rather than one function: every
verb the action leader offers in every pane that offers it, every command the
`:` bar registers, every modal driven through the editing keys and not just
navigation, and every background message claimed by exactly one async handler.
The AST check already proved a case existed for each message; that one proves
it runs. The modal sweep found `RemapModal.selectOption` reading past the end
of its error list once the wizard had decided the last one.

Then the layer none of that reaches. Everything the tool learns about GitHub
arrives as bytes from a `gh` subprocess, and a fixture of those bytes records
what its author imagined. `internal/github` and `internal/app` now replay real
recordings through `aragonite/ghcassette`: every read method on the client, and
the whole path from a branch's state through a run's timeline to its log.
`AGENTS.local.md` holds what a cassette test has to do to be worth its bytes.

That work needed one product fix. The mutation guard blocked every `gh pr`
call, so `gh pr list` (how a pull request's check rollup is read) panicked in
tests and `PullRequestsInScope` could not be exercised at all. Operations are
now allowlisted per name, so a read gets through and anything unrecognized
stays blocked.

## Not done

Found this phase, not fixed:

- **A pull request listing ignores the repository it was asked for.**
  `PullRequestsInScope` passes the client's `owner/repo` to aragonite, which
  uses it only as a cache key: the `gh pr list` behind it resolves its
  repository from the process's working directory instead. The TUI happens to
  agree, because you launch it inside the repository you are reading, so
  nothing is visibly wrong today. It is still an unstated coupling, and the
  cassette test has to stand the process in a throwaway checkout to record
  against anything else. The fix is a `--repo` flag in aragonite's
  `prListPage`, which means an upstream change and a release
- **`gofumpt`'s `extra-rules` is deprecated.** `golangci-lint` warns on every
  run. The replacement, `extra.group-params`, is not the same rule:
  `my_go_template` measured 31 diff lines against plain gofumpt for
  `extra-rules` and 12 for `group-params`, so taking it relaxes formatting and
  forces a reformat commit in every child project. The template records that
  decision; this repository just inherits the warning
- **A local edit to `.typos.toml`.** Cassettes are excluded there because a
  UUID inside one reads as a misspelling. The file is template-managed, so the
  edit returns as a `.rej` on the next `copier update` and has to be re-applied
- **Coverage is uneven under the line.** `internal/ui/panes` and
  `internal/ui/modal` still hold the largest untested blocks, mostly in the
  render paths that the contract tests exercise without asserting on. The
  module clears 85% overall; no package floor is enforced
- **`internal/runner` and the dispatch itself stay untested end to end.**
  Dispatching is a mutation, so no cassette can record it and the guard panics
  rather than let a test try. `mise run test:live` is the only thing that
  covers it, and it spends Actions minutes

## Scrapped

Both were filed as v2 alternatives and neither survived contact with what
shipped:

- **Single-screen dashboard.** It was an alternative to the modal-stack UX, and
  Phase 8 removed most of the modal stack's job: details live in panes, the
  timeline is a drill-down, the layout is lazygit's two-column shape. A second
  view layer would fork maintenance against a problem that largely went away
- **SQLite-backed frecency store.** It buys queries nobody runs. The JSON file
  is one user's history on one machine, and nothing has measured a load cost

## Still deferred

- Ranking failure signatures rather than reporting each once per step. Scoping
  the scan to a failed step's last 200 lines removed the false positives that
  made this look urgent, so it stays deferred until a real corpus shows noise
  again. `AGENTS.local.md` holds how to measure that
