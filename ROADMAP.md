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

Not built, and worth doing only if the tab proves useful: redrawing as a
watched run progresses. The layout already handles a span with no end by
closing it at `now`, so what is missing is a redraw tick rather than any new
arithmetic.

## Phase 4: keyboard composability (shipped)

`a` opens the verbs scoped to whatever has focus, and `:` opens a command bar
with tab completion. Ported from
[gh-repo-dashboard](https://github.com/KyleKing/gh-repo-dashboard), which keeps
the two as separate grammars on purpose: a menu is a list you read, a bar is a
name you type.

Not built: marks and operator-object batch verbs (`!d` over a marked set).
`space` is bound to focus-config, so adopting them means moving that first.

## Deferred / v2 ideas

Tracked in `DESIGN.md` and `CONTRIBUTING.md` design-decision sections:

- Single-screen dashboard alternative to the modal-stack UX
- SQLite-backed frecency store (currently JSON)
- `environment`-type input resolution via repo-environments API call
