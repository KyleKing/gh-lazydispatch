# Template Adoption: Next Steps

Context for the 2026-07-03 adoption of unreleased my_go_template fixes (commit c24db4a). Both repos still have loose ends to close out.

## Problem

gh-lazydispatch predates several template conventions: it had no mise tasks, no golangci v2 config, no commitizen setup, and no hk-managed bump workflow. Pulling those in surfaced a template bug (mise never loaded `.config/mise.template.toml`, since mise only reads `mise.toml` plus `mise.$MISE_ENV.toml`), which was fixed in the template first (`.config/mise/conf.d/`, always loaded regardless of `MISE_ENV`) and then adopted here.

Adopting golangci v2 also means lint has effectively never run on this repo: the old v1 config silently failed to load ("typecheck is not a linter"), so lint was a no-op in CI. The new config is valid and surfaces 572 pre-existing findings.

## Decisions made

- **Template vs. repo fix**: the conf.d bug was fixed upstream in my_go_template, not worked around locally, since it affects every generated project. This repo adopted the fixture output directly rather than duplicating the fix.
- **Dual bump workflows**: `version-bump.yml` and `ci-gate.yml` stay, since they're this repo's own dispatchable demo workflows (documented in `docs/chain-examples.md`, referenced by `testdata/.github/lazydispatch-release.yml`). The template's `bump_version.yml` was added alongside them rather than replacing them, so the repo now has both a manual/dispatchable bump path (the app's own feature demo) and an automatic commitizen-driven bump on push to main.
- **Lint debt**: rather than block this adoption on fixing 572 findings, they're deferred to a follow-up pass (see below).

## Open decisions / follow-ups

### Dual bump workflow risk

The new `bump_version.yml` auto-bumps and tags a release on the next push to main. Confirm this is wanted before the next push, since it will fire alongside the existing `version-bump.yml`/`ci-gate.yml` demo flow. If the auto-bump isn't wanted yet, disable or gate it before pushing.

### Template release and copier update

`.copier-answers.yml` is still pinned to `_commit: v0.2.2`, which predates the conf.d fix. Once my_go_template tags a new release, run `copier update` here to pick it up cleanly instead of the manual adoption done in c24db4a.

### go-size-analyzer (gsa) build failure

`mise run ci`/`mise run lint` fail locally because `go:github.com/Zxilly/go-size-analyzer/cmd/gsa@latest` (pinned to 1.13.0 in the template's `mise.hk.toml`) doesn't build on Go 1.26.4 (needs `GOEXPERIMENT=jsonv2` for `encoding/json/v2`). Workaround until upstream fixes it or the template pins/drops it: `MISE_ENV=ci mise run ci` skips hk tool bootstrap entirely, or invoke `golangci-lint` directly.

### Lint debt (572 findings)

Lint never actually ran before (the v1 config failed to load), so this is a full backlog, not a regression. Top categories: revive, govet, gocritic, mnd, paralleltest (~50 each), lll (46), gocognit/goconst (20 each), testpackage (19), wrapcheck (16). A subagent has been dispatched to work through these iteratively; see commit history after this doc for progress.

### Upstreaming candidates

my_go_template's `NEXT_STEPS.md` already tracks pulling gh-lazydispatch's `test:integration`/`bench:*` task conventions and test-safety patterns into the shared template. No action needed here beyond what's already tracked upstream.
