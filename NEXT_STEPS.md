# Next steps

Findings from a review pass on 2026-07-21, reading the keymap and dispatch paths and driving the TUI headlessly. Each item names the symbol to change and a suggested approach. Line numbers are omitted because they drift; grep the symbol.

## Already fixed

- The TUI panicked with `index out of range [-1]` when navigating up past the first workflow after touching the config pane. `viewInputDetailsPane` in `internal/app/views.go` now uses the guarded `SelectedWorkflow()` accessor
- `scripts/live-test.sh` (`mise run test:live`) drives the real dispatch path against a scratch repo and confirms the values the workflow received match what the tool sent

## Keymap, most visible to a new user

`l` is bound twice and the global binding wins. `LiveView` and `TabNext` in `internal/app/keymap.go` both bind `l`, and `handleGlobalKey` matches `LiveView` first and returns handled unconditionally. That kills the README's "press `l` for the Chains tab" and "press `l` to view logs" for a history entry; only the right arrow reaches the Chains tab, so tab cycling is asymmetric because `h`/`left` still work. Rebind the live-view modal to a free key, or let `handleGlobalKey` fall through when the history pane is focused.

Every number key for workflow selection is dead code. `handleKeyMsgFallback` in `internal/app/handlers.go` loops `InputKeys()` first and returns unconditionally, and `InputKeys()` and `WorkflowKeys()` bind the same characters `0`-`9`. So `handleWorkflowKey` is never reached, and `0` ("workflow all", per the help text) does nothing. Take the input branch only when `focused == PaneConfig`, which is the condition `handleInputKey` already checks internally before it no-ops.

`Enter` in the workflows pane does nothing. `handleEnter` in `internal/app/handlers.go` has an empty `case PaneWorkflows:`, while the footer renders `[Enter] run` and `--help` says "Select / Execute workflow". Either wire it to the same path as the config pane's Enter, or stop advertising it.

## Behavior

A malformed workflow file is reported as "no workflows found". `Discover` in `internal/workflow/discovery.go` swallows every parse error with a bare `continue`, so one typo in a YAML file looks identical to an empty repo. Collect the failures and render them in the empty state, which is already well built for the chains case.

`environment`-type inputs silently degrade to a free-text box. Parsing keeps the type string, but `handleInputKey` only special-cases `boolean` and `choice`, so `environment` falls through to a plain input modal with no validation and no list. This is on the ROADMAP as deferred, so it is a known gap. The smaller interim fix is to label it as unsupported in the modal rather than presenting it as a string.

Modal results are delivered from a goroutine and dropped silently if a keystroke races them. The live-test work hit this: keystrokes sent too quickly landed while the previous modal's result was still in flight, `Update` routed the stale result into the newly-pushed modal, and three input edits vanished with no error surface. A fast typist could hit the same thing. At minimum log or surface a dropped result rather than discarding it; the harness works around it by waiting for each value to appear in the command preview.

One sibling of the fixed panic remains half-guarded. `renderTableRows` in `internal/app/views.go` checks only the upper bound on `selectedWorkflow`. It is currently unreachable with `-1` because `viewConfigPane` fully guards before calling it, so it is safe today. Use the `SelectedWorkflow()` accessor there too, so the invariant does not depend on a caller.

## Release and distribution

The extension is installable as of v1.0.5. Two separate defects had to be cleared. First, `bump_version.yml` pushed the tag with `secrets.GITHUB_TOKEN`, and GitHub suppresses workflow triggers for `GITHUB_TOKEN`-authored pushes, so the separate `release.yml` never ran and v1.0.0 through v1.0.4 carried zero assets. Goreleaser now runs as a step inside the `bump_version.yml` job, guarded on `env.REVISION != env.PREVIOUS_REVISION`, and `release.yml` is deleted. Second, `builds[].no_unique_dist_dir` sent all ten targets to the same `dist/gh-lazydispatch` path, so they overwrote each other and the assets would have been ten copies of whichever build finished last. That key is gone.

Template v0.7.0 added an unconditional `homebrew_casks` block to `.goreleaser.yml` and `TAP_DEPLOY_KEY` to the inline goreleaser step. `TAP_DEPLOY_KEY` is not provisioned on this repo, so `skip_upload` resolves to `true`, the release publishes its binaries, and no cask is pushed. Run `scripts/provision-tap-deploy-key.sh` when the tap should start receiving casks.

That makes `Formula/gh-lazydispatch.rb` and the `brew:sha` mise task redundant. The task exists only to tell you to paste SHA256 values into a stub that goreleaser now generates on every release. Both should go, but the removal belongs in the template so every child loses them in one update, not here.

v1.0.5 is verified from the published artifacts rather than the asset count: ten binaries with ten distinct checksums and ten distinct sizes, and `file` confirms Mach-O arm64, statically linked ELF x86-64, and PE32+ across the platforms it should. `gh extension install kyleking/gh-lazydispatch` resolves and `gh lazydispatch --version` runs, because the asset names match the `<extension-name>-<os>-<arch>[.exe]` convention gh derives from the repo name.

The five asset-less releases are deleted, so nothing advertises an install that returns nothing. Their tags are kept on purpose: proxy.golang.org has already cached v1.0.0 through v1.0.4 permanently, so `go install ...@v1.0.4` resolves from the proxy no matter what the repo does, and dropping the tags would only make the repo and the proxy disagree.

`Formula/gh-lazydispatch.rb` ships `version "0.1.0"` with `REPLACE_WITH_SHA256_FOR_*` placeholders, and `kyleking/homebrew-tap` returns 404. It also builds the download filename with `Hardware::CPU.arch` (`x86_64`) while the release URLs use `amd64`, so an Intel install would fail even with real SHAs. `.goreleaser.yml` also has no `brews`/`homebrew_casks` block, so nothing would publish to the tap even if it existed. Deferred pending the manual tap setup; until then the README advertises a `brew install` that 404s.

## Lint

Resolved on 2026-07-30: `hk fix --all` now exits 0 over all 184 files. The SC2086 findings that used to fail it came from actionlint's embedded shellcheck on `.github/workflows/bump_version.yml`, and the template rewrote that file at v0.7.0.

Resolved on 2026-07-30: template v0.7.0 deleted the `toml-sort-fix` block from `.pre-commit-config.yaml`, so hk's `tombi-format` is the only TOML formatter and `.cz.toml` and `.golangci.toml` now reach a fixed point. Three consecutive `hk fix --all` runs leave both byte-identical.

hk's config-based hooks are installed alongside prek rather than replacing it, because dropping prek would lose checks `hk.pkl` has no equivalent for. What prek runs here and hk does not: `copier-forbidden-files` (the `.rej` guard the copier workflow depends on), `shellcheck` on standalone scripts, `mdformat`, `prettier` over html/js/json/shell/yaml, `check-executables-have-shebangs`, `check-json`, `check-toml`, `check-vcs-permalinks`, `debug-statements`, `destroyed-symlinks`, `fix-byte-order-marker`, and `forbid-new-submodules`. hk adds `actionlint`, `ls-lint`, `pkl`, `yamllint`, `tombi-format`, and a pre-push `mise run ci` that prek never ran, so it is a trade. Backfilling the missing checks into the template's `hk.pkl` is the open decision; until then both hook systems fire and, with toml-sort gone, they no longer collide.

hk's `newlines` step has no `.copier-answers.yml` exclusion where prek's `end-of-file-fixer` does, so hk strips the trailing blank line copier writes and every update produces a spurious one-line diff. Known, headed upstream to the template.

## Template and dependencies

On copier `my_go_template` v0.7.0, current. Bubbletea is already on `charm.land/*/v2`, so no framework migration is needed. See `.freshen.md` for the per-update log.

No dependabot PRs are open. #10 (go modules) and #15 (action pins) are merged; #14 was closed because it edited the deleted `release.yml` and #15 superseded its pins.

## Running the live test

The harness needs a token that can create and delete repositories, which the default fine-grained PAT usually cannot. See the "Token setup" section in `docs/live-testing.md`.
