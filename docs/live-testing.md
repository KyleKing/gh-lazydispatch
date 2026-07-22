# Live dispatch testing

The unit suite can never issue a real `gh workflow run`. `internal/exec` panics on any mutating `gh` command while `testing.Testing()` is true, and `scripts/check-test-safety.sh` fails the build on test files that reach for a real executor. That is the right default, and it leaves one gap: the tool's entire job is building and issuing a dispatch, and that path is only ever exercised against mocks.

`scripts/live-test.sh` closes the gap. It creates a throwaway GitHub repository, pushes a `workflow_dispatch` workflow that echoes its inputs, drives the TUI headlessly against it, and checks that the values the TUI sent are the values the workflow received.

## Cost

**This test consumes GitHub Actions minutes.** One run dispatches a single `ubuntu-latest` job that runs three `echo` commands, so the billed time is roughly a minute including runner startup. It also creates a private repository under your account and deletes it again. Do not wire it into CI on every push.

## Running it

```bash
GH_LD_LIVE_TEST=1 mise run test:live
# or, directly
./scripts/live-test.sh --yes
# keep the scratch repo and temp dir for debugging
KEEP=1 ./scripts/live-test.sh --yes
```

Without `--yes` or `GH_LD_LIVE_TEST=1` the script prints what it would create and destroy, then exits 1. Nothing happens by accident.

Prerequisites: `gh`, `git`, and `go` on `PATH`, and a `gh` token that can create **and delete** repositories. Classic tokens need the `delete_repo` scope (`gh auth refresh -h github.com -s delete_repo`); fine-grained PATs need `Administration: write` on your account.

## What it does

1. Checks `gh auth status`, resolves the account, and verifies the token can delete repositories.
1. Builds `./cmd/gh-lazydispatch` from source into the temp directory. The test never uses an installed extension.
1. Creates a private repo named `gh-ld-livetest-<epoch>-<rand>`, clones it into a `mktemp -d`, and pushes `.github/workflows/livetest.yml` with one input of each interesting type: `level` (choice), `message` (string), `verbose` (boolean). The job echoes all three as `LIVETEST_*` lines.
1. Enables Actions on the scratch repo and waits for GitHub to register the workflow.
1. Runs `go test -tags live -run TestLiveDispatch ./internal/app/` with the scratch repo details in the environment.
1. Confirms the dispatched run with `gh run list`, waits for it to finish with `gh run watch --exit-status`, downloads the logs with `gh run view --log`, and greps for `LIVETEST_LEVEL=warn`, `LIVETEST_MESSAGE=live-<nonce>`, and `LIVETEST_VERBOSE=true`.
1. Deletes the scratch repo and the temp directory.

## Why the TUI is driven, not the command string

The tool has no CLI dispatch subcommand, so there are two ways to test the real path: assert on the command string the model builds and then run that string from the shell, or drive the TUI itself. The second is what `internal/app/live_internal_test.go` does, because the project already uses `teatest` (see `teatest_internal_test.go`) and because `tea.ExecProcess` sets the child's stdout to the program's output writer, which under `teatest` is a buffer the test can read. The real `gh workflow run` therefore runs inside the real bubbletea event loop, triggered by the real confirmation modal, and its success line is observable from the test.

Asserting on `buildCLIString()` alone would have skipped the modal stack, the `RunConfirmResultMsg` round trip, and `doExecuteWorkflow`. The live test does both: it asserts the built command *and* dispatches through the event loop.

The test replays keystrokes rather than setting model fields:

| Keys                         | Effect                                  |
| ---------------------------- | --------------------------------------- |
| `tab tab`                    | Focus the config pane                   |
| `0`, `↓`, `enter`            | Choice input `level`: `info` → `warn`   |
| `1`, `ctrl+u`, type, `enter` | String input `message` → `live-<nonce>` |
| `2`, `y`                     | Boolean input `verbose` → `true`        |
| `enter`                      | Open the run confirmation               |
| `y`                          | Dispatch for real                       |

Inputs are numbered in sorted order, which is why `level` is `0`.

## Safety design

The script is written so that a crash at any point cannot damage anything you own.

- **Opt-in only.** No flag, no run. The refusal message lists every create and destroy it would perform.
- **It only ever touches a repo it created.** The name is `gh-ld-livetest-<epoch>-<rand>`, and the `EXIT` trap re-checks the `gh-ld-livetest-` prefix before calling `gh repo delete`. A name without the prefix is reported and left alone.
- **All work happens in `mktemp -d`.** The script asserts `$PWD` is under that directory before the first write, and the trap refuses to `rm -rf` a path that is not under a temp directory.
- **Cleanup runs on failure too**, via `trap ... EXIT`, unless `KEEP=1`.
- **Delete permission is verified before the scratch repo exists.** Classic tokens are checked against the advertised `delete_repo` scope. Fine-grained PATs do not advertise scopes, so the script creates and deletes an empty probe repo first and aborts if the delete fails, rather than discovering the problem after there is something worth leaking. A delete that fails anyway prints `LEAKED REPOSITORY` with the name so you can remove it by hand.
- **Actions are enabled explicitly** and the setting is read back, so a repo where a dispatch could never run fails loudly instead of hanging.

The Go side has its own guard: `TestLiveDispatch` skips unless `GH_LD_LIVE_REPO` is set, and fails outright if that repo name lacks the `gh-ld-livetest-` prefix. It is behind the `live` build tag, so `go test ./...` does not compile it and the mock-executor safety net in `internal/exec` stays intact for every other test.

## Troubleshooting

`token cannot delete repositories` — refresh the token as described above.

`could not enable GitHub Actions` — an organization or enterprise policy is blocking Actions on new repositories. Run the test under a personal account.

`no workflow_dispatch run appeared` — GitHub occasionally takes longer than the poll window to surface a run. Re-run with `KEEP=1` and inspect the scratch repo.
