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
is right.

## Phase 5: the branch's current state, not this checkout's history

The History tab lists local dispatch history, so in a repository this tool has
never dispatched from it reads "No recent runs" while GitHub holds thousands. The
question a reader actually opens the tool with is whether their branch is green,
and nothing on screen answers it.

`aragonite/forge/github.LatestRunsOnBranch` is the read this needs: the newest
run of each workflow on a ref, keyed on workflow file *and* display title so a
workflow reporting its mode in the title (a Pulumi preview against a Pulumi
deploy) keeps one current state per mode. Runs older than a cutoff drop out
unless they are still going.

Still to build:

- A Runs pane over that read, lazily loaded, with the frecency History tab left
  alone. Filters worth having beyond the current branch: my open PRs, PRs
  awaiting my review. Both are `gh search prs` reads this tool does not make yet
- A flakiness view. Per-branch, one run per workflow is the whole answer; asking
  whether a workflow is flaky is the opposite query (many runs of one workflow)
  and wants its own screen rather than a column
- Deciding the age cutoff. Four hours is right for a busy repository and wrong
  for a quiet one, so it likely reads as "the last run, plus anything from the
  last N hours", with N falling back to three runs when the repository is quiet

## Phase 6: layout from first principles

The right panel hides four tabs' worth of information behind `h`/`l`, so the
counts a reader wants at a glance cost a keystroke each and are invisible until
then. The redesign should decide what earns permanent screen space at 80x24
before deciding what any pane looks like, and degrade upward to a wide terminal
rather than the reverse.

Constraints already learned the hard way: no pane may size itself to its content
(`ui.PaneStyle` now clamps with `MaxHeight`, and every pane needs a scroll window
like `scrollWindow` and `renderTableRows` have), and the footer is load-bearing
because the command bar draws on it.

Golden frames at 80x24, 120x40, and 160x50 with a realistic workflow count are
the acceptance test, alongside `assertFits`, which already catches overflow once
a fixture is large enough to trigger it.

## Phase 7: aragonite consolidation

`tui/theme`, `tui/table`, and `ghcassette` were already shared.
`aragonite/forge/github` now holds the Actions reads (run by ID, run list by
query, jobs with per-step timings, latest run per workflow on a ref) and exports
`WithRunner`, which is what lets this repository keep `internal/exec`'s mutation
guard in front of every gh call rather than reinstating it per call site.

Done, uncommitted, and blocked on an aragonite release: `internal/github`'s read
paths delegate through that seam. This repository cannot compile against it until
aragonite tags a version carrying the new API, so `go.mod` still names v0.3.0 and
the change only builds with a `go.work` pointing at a sibling checkout.

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
