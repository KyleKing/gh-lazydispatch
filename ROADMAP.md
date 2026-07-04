# Roadmap

Phased plan for gh-lazydispatch after the v1.0.0 release (2026-07-04). Consolidates the loose notes that previously lived in `NOTES.md`, `TEMPLATE_ADOPTION_NEXT_STEPS.md`, and the README "Planned Features" list.

Status legend: [ ] not started, [~] in progress, [x] done.

## Phase 0: Housekeeping (in progress)

Close out the template-adoption follow-ups and tidy stray notes now that v1.0.0 shipped.

- [x] Clear the golangci v2 lint backlog (was 572 findings, now 0)
- [x] Fix the gsa build failure (installed via brew, dropped from mise pin)
- [x] Resolve the dual bump-workflow risk (`bump_version.yml` is the sole release path; `version-bump.yml`/`ci-gate.yml` remain dispatch-only demo content)
- [ ] Run `copier update` to adopt template v0.3.0, replacing the manual v0.2.2 adoption in `c24db4a` (`.copier-answers.yml` is still pinned to `_commit: v0.2.2`). Template v0.3.0 was tagged 2026-07-04 and includes the `conf.d` fix. Do this as an isolated commit after the housekeeping cleanup lands, so any conflict resolution is easy to review.

## Phase 1: Markdown log export (next)

Export fetched run/chain logs to a markdown file. This is the smallest of the planned features and reuses existing structures.

Starting points:

- `internal/logs/types.go` already models the data: `RunLogs` (chain name, branch, steps), `StepLogs` (workflow, job, status, conclusion, entries), and `LogEntry` (timestamp, content, level, step name)
- `internal/chain/export.go` (`ExportAsBash`) is the pattern to follow: a pure function that takes domain structs and returns a formatted string
- `internal/logs/filter.go` can produce a filtered `[]LogEntry` so export can honor the active filter (all / errors / warnings)

Scope:

- Add `ExportAsMarkdown(*logs.RunLogs, *logs.FilterConfig) string` (pure, testable, no I/O), grouping by step with a per-step status/conclusion header and fenced code blocks for log bodies
- Wire an export keybinding into the log viewer modal (see `internal/ui/modal/`), writing to a file path and surfacing the result via the existing error/status message pattern
- Table-driven tests covering empty runs, filtered output, and multi-step chains

## Phase 2: Error pattern detection

Detect common failure signatures in logs (timeouts, OOM/killed, permission/auth errors, missing-secret) and surface them, e.g. a summary line or jump-to-first-match in the log viewer.

Builds on `internal/logs/filter.go` and `LogLevelError`. Define patterns as data (regex + label + hint) so the set is easy to extend. Consider reusing this in Phase 1 export output (a "Detected issues" section).

## Phase 3: Timeline view

Visual timeline for run/log visualization (largest UI effort). New rendering in `internal/ui/`, likely a new pane or modal. Design the layout in `UX.md` before implementing. Deferred until Phases 1-2 validate the log-data model under real use.

## Deferred / v2 ideas

Tracked in `DESIGN.md` and `CONTRIBUTING.md` design-decision sections:

- Single-screen dashboard alternative to the modal-stack UX
- SQLite-backed frecency store (currently JSON)
- `environment`-type input resolution via repo-environments API call
