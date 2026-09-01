# Roadmap

Phased plan for gh-lazydispatch after the v1.0.0 release (2026-07-04). Open defects and immediate follow-ups live in `NEXT_STEPS.md`.

Phase 0 (housekeeping: the golangci v2 backlog, the gsa build failure, the dual bump-workflow risk, and the copier adoption) is complete and is not tracked here.

Phases 1 (markdown log export) and 2 (error pattern detection) shipped together,
because the export's "Detected issues" section is the detector's first consumer.
`internal/logs/export.go` holds `ExportAsMarkdown` and `internal/logs/patterns.go`
holds the signature set; `x` in the log viewer writes the file.

## Phase 3: Timeline view (next)

Visual timeline for run/log visualization (largest UI effort). New rendering in `internal/ui/`, likely a new pane or modal. Design the layout in `UX.md` before implementing.

## Deferred / v2 ideas

Tracked in `DESIGN.md` and `CONTRIBUTING.md` design-decision sections:

- Single-screen dashboard alternative to the modal-stack UX
- SQLite-backed frecency store (currently JSON)
- `environment`-type input resolution via repo-environments API call
