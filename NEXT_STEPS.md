# Next steps

Open defects and follow-ups. Phased feature work lives in `ROADMAP.md`; the
append-only pass log is `.freshen.md`. Every item was re-verified against the
source on 2026-09-01. Each names the symbol to change rather than a line number,
because line numbers drift.

## Behavior

Every failure signature `logs.Detect` reports is a regular expression over log
text, so a test named "handles timeouts" matches `Timeout`. Each detection
carries the line that matched, which is what lets a reader see it is a false
positive, but nothing ranks or scores them. If the noise becomes a problem, the
fix is to weight a signature by where in the step it matched rather than to
narrow the patterns.

`.golangci.toml` sets gofumpt's `extra-rules`, which golangci-lint 2.13.2
deprecates in favor of `extra.group-params`, so every format run prints a
warning. The file is template-managed and
[my_go_template](https://github.com/KyleKing/my_go_template) has already
measured the swap and decided against it: the two keys are not equivalent, and
taking the replacement means a reformat commit in every child. The warning
stands until that is worth doing.

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

On copier `my_go_template` v0.15.1. Bubbletea is already on `charm.land/*/v2`, so
no framework migration applies. No Dependabot PRs are open.

`README.md`, `DESIGN.md`, `go.mod`, `.config/mise.toml`, and
`cmd/gh-lazydispatch/main.go` sit in the template's `_skip_if_exists`, so copier
never touches them and each has diverged from the seed on purpose. Any future
template change to those five needs a manual diff against
`my_go_template/.ctt/default/`. `AGENTS.md` left that list, so `copier update`
keeps it current on its own.

`.typos.toml` excludes `**/testdata/cassettes/*.golden`, which is a local edit
to a template-managed file and will come back as a `.rej` on the next `copier
update`. Re-apply it: a cassette is a byte-exact recording of what gh printed,
and a UUID inside one reads as a misspelling that must not be corrected.

## Running the live test

`scripts/live-test.sh` (`mise run test:live`) drives the real dispatch path
against a scratch repo and confirms the values the workflow received match what
the tool sent. It needs a token that can create and delete repositories, which
the default fine-grained PAT usually cannot. See the "Token setup" section in
`docs/live-testing.md`.
