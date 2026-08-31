# Next steps

Open defects and follow-ups. Phased feature work lives in `ROADMAP.md`; the
append-only pass log is `.freshen.md`. Every item was re-verified against the
source on 2026-08-01. Each names the symbol to change rather than a line number,
because line numbers drift.

## Keymap, most visible to a new user

`l` is bound twice and the global binding wins. `LiveView` and `TabNext` in
`internal/app/keymap.go` both bind `l`, and `handleGlobalKey` matches `LiveView`
first and returns handled unconditionally. That kills the README's "press `l` for
the Chains tab" and "press `l` to view logs" for a history entry; only the right
arrow reaches the Chains tab, so tab cycling is asymmetric because `h`/`left`
still work. Rebind the live-view modal to a free key, or let `handleGlobalKey`
fall through when the history pane is focused.

Every number key for workflow selection is dead code. `handleKeyMsgFallback` in
`internal/app/handlers.go` loops `InputKeys()` first and returns unconditionally,
and `InputKeys()` and `WorkflowKeys()` bind the same characters `0`-`9`. So
`handleWorkflowKey` is never reached, and `0` ("workflow all", per the help text)
does nothing. Take the input branch only when `focused == PaneConfig`, which is
the condition `handleInputKey` already checks internally before it no-ops.

`Enter` in the workflows pane does nothing. `handleEnter` in
`internal/app/handlers.go` still has an empty `case PaneWorkflows:`, while the
footer renders `[Enter] run` and `--help` says "Select / Execute workflow".
Either wire it to the same path as the config pane's Enter, or stop advertising
it.

## Behavior

A malformed workflow file is reported as "no workflows found". `Discover` in
`internal/workflow/discovery.go` swallows every parse error with a bare
`continue`, so one typo in a YAML file looks identical to an empty repo. Collect
the failures and render them in the empty state, which is already well built for
the chains case.

`environment`-type inputs silently degrade to a free-text box. Parsing keeps the
type string, but `handleInputKey` only special-cases `boolean` and `choice`, so
`environment` falls through to a plain input modal with no validation and no
list. `ROADMAP.md` defers the real fix (a repo-environments API call). The
interim fix is to label it unsupported in the modal rather than presenting it as
a string.

Modal results are delivered from a goroutine and dropped silently if a keystroke
races them. The live-test work hit this: keystrokes sent too quickly landed while
the previous modal's result was still in flight, `Update` routed the stale result
into the newly-pushed modal, and three input edits vanished with no error
surface. A fast typist could hit the same thing. At minimum log or surface a
dropped result rather than discarding it; the harness works around it by waiting
for each value to appear in the command preview.

One sibling of the fixed `index out of range [-1]` panic remains half-guarded.
`renderTableRows` in `internal/app/views.go` checks only the upper bound
(`m.selectedWorkflow >= len(m.workflows)`). It is unreachable with `-1` today
because `viewConfigPane` fully guards before calling it, so it is safe, but the
invariant depends on the caller. Use the `SelectedWorkflow()` accessor there too.

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
