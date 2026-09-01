# Next steps

Open defects and follow-ups. Phased feature work lives in `ROADMAP.md`; the
append-only pass log is `.freshen.md`. Every item was re-verified against the
source on 2026-08-31. Each names the symbol to change rather than a line number,
because line numbers drift.

## Behavior

`environment`-type inputs still resolve to a free-text box. The modal now says
the value is sent unresolved, which is the interim fix; the real one is a
repo-environments API call, deferred in `ROADMAP.md`.

Every failure signature `logs.Detect` reports is a regular expression over log
text, so a test named "handles timeouts" matches `Timeout`. Each detection
carries the line that matched, which is what lets a reader see it is a false
positive, but nothing ranks or scores them. If the noise becomes a problem, the
fix is to weight a signature by where in the step it matched rather than to
narrow the patterns.

## Interface

A repository that configures no chains now draws no chains pane, which also
removed the placard explaining what chains are and how to configure one. The
feature is documented in `docs/chains.md` and reachable through `:chain`, so
nothing is unreachable, but a first-time reader no longer trips over it.

## Hooks

`.config/mise/mise.lock` is the lockfile CI installs from, and `mise install
--locked` fails on any tool missing from it. Adding a tool to
`.config/mise/conf.d/*.toml` means running `mise lock --platform
linux-x64,macos-arm64` in the same commit. Only those two platforms are locked,
so a contributor on linux-arm64 or windows cannot use `--locked`.

## Release and distribution

The extension is installable as of v1.0.5, verified from the published artifacts:
ten binaries with ten distinct checksums and ten distinct sizes, `file` confirming
Mach-O arm64, statically linked ELF x86-64, and PE32+ across the platforms it
should, `gh extension install kyleking/gh-lazydispatch` resolving, and
`gh lazydispatch --version` running.

Tags v1.0.0 through v1.0.4 are kept on purpose even though their assetless
releases were deleted: proxy.golang.org has cached them permanently, so
`go install ...@v1.0.4` resolves from the proxy no matter what the repo does, and
dropping the tags would only make the repo and the proxy disagree.

Open: `TAP_DEPLOY_KEY` is provisioned on this repo but `kyleking/homebrew-tap`
holds no `gh-lazydispatch` cask yet, because no release has been cut since the
secret was added. The next release should push one; confirm it does rather than
assuming.

## Template and dependencies

On copier `my_go_template` v0.13.0. Bubbletea is already on `charm.land/*/v2`, so
no framework migration applies. No Dependabot PRs are open.

`README.md`, `DESIGN.md`, `go.mod`, and `cmd/gh-lazydispatch/main.go` sit in the
template's `_skip_if_exists`, so copier never touches them and each has diverged
from the seed on purpose. `AGENTS.md` is in the same list but was still carrying
the pre-v0.6 shape; the v0.9.1 rewrite was copied over by hand. Any future
template change to those five needs the same manual diff against
`my_go_template/.ctt/default/`.

## Running the live test

`scripts/live-test.sh` (`mise run test:live`) drives the real dispatch path
against a scratch repo and confirms the values the workflow received match what
the tool sent. It needs a token that can create and delete repositories, which
the default fine-grained PAT usually cannot. See the "Token setup" section in
`docs/live-testing.md`.
