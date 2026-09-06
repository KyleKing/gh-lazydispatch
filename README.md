# gh-lazydispatch

![demo](https://raw.githubusercontent.com/KyleKing/gh-lazydispatch/main/.github/assets/demo.gif)

Pick a GitHub Actions workflow, fill in its `workflow_dispatch` inputs, and run it without leaving the terminal. Chain several workflows into one sequence and read the logs as they stream back.

## Alternatives

Other tools that trigger Actions runs, read their logs, or run them somewhere other than a GitHub-hosted runner:

| Tool | Notes |
| --- | --- |
| [act](https://github.com/nektos/act) | "Think globally, `act` locally" |
| [action-tmate](https://github.com/mxschmitt/action-tmate) | "Debug your GitHub Actions by using tmate" |
| [Buildkite Test Engine](https://buildkite.com/platform/test-engine/) | "Deliver confidently with fast and reliable tests" (Closed source) |
| [Datadog CI Visibility](https://www.datadoghq.com/product/ci-cd-monitoring/) | "Monitor & improve the performance of your business-critical builds & deployments in any CI/CD tool" (Closed source) |
| [gh](https://github.com/cli/cli) | "GitHub CLI brings GitHub to your terminal", including the built-in `gh workflow run` and `gh run view --log-failed` |
| [gh-act](https://github.com/nektos/gh-act) | "GitHub CLI Extension to run GitHub actions locally using nektos/act" |
| [gh-annotations](https://github.com/swfz/gh-annotations) | "list of annotations from the recently executed Workflow" |
| [gh-dash](https://github.com/dlvhdr/gh-dash) | "A rich terminal UI for GitHub that doesn't break your flow" |
| [gh-dispatch](https://github.com/mdb/gh-dispatch) | "A gh CLI extension for sending GitHub dispatch events and watching the triggered GitHub Actions runs directly from your terminal" |
| [ghui](https://github.com/kitlangton/ghui) | "Terminal UI for keeping up with your open GitHub pull requests across repositories", with a run and job drill-down for the checks on one |
| [Mergify CI Insights](https://mergify.com/product/ci-insights) | "Your CI is a black box. Open it up." (Closed source) |
| [Trunk Flaky Tests](https://trunk.io/flaky-tests) | "Trunk detects, quarantines, and eliminates flaky tests from your code base" (Closed source) |
| [wrkflw](https://github.com/bahdotsh/wrkflw) | "A command-line tool for validating and executing GitHub Actions workflows locally" |
| _[Awesome Actions](https://github.com/sdras/awesome-actions)_ | "A curated list of awesome actions to use on GitHub" |

## Install

```bash
# GitHub CLI extension
gh extension install kyleking/gh-lazydispatch
# Go
go install github.com/kyleking/gh-lazydispatch/cmd/gh-lazydispatch@latest
```

## Quick start

From a git repository that has workflows:

```bash
cd your-project
gh lazydispatch
```

It finds every workflow with a `workflow_dispatch` trigger and lists them. `tab` moves between panes, `enter` runs the highlighted workflow, and `?` opens the keymap.

When you do not remember a key, `a` lists the actions that apply to whatever has focus and `:` opens a command bar that completes over this repository's own branches, chains, and workflows.

## Reading a failed run

`gh run view --log` returns the whole log, and `--log-failed` still returns every line of every failed job. `export diagnose` parses it once and returns the failure:

```bash
gh lazydispatch export diagnose 33423560774
```

On a real CI failure in this repository that is 2,361 bytes against 237,144 through `gh run view --log-failed`. It reports which steps failed, the error lines each one logged, a window of context ending at the failure, and any known failure signature it matched.

The other read-only commands are `export logs`, `export runs`, `export workflows`, and `export chains`. Every one writes JSON to stdout and its line counts to stderr; none of them dispatch anything. See [docs/cli.md](./docs/cli.md), and [skills/github-actions](./skills/github-actions) for the agent-facing version.

`gh lazydispatch watch <run-id>` chains after a push: it blocks until the run finishes, writes an errors-only digest to disk, and prints the path. `--fix` hands that digest to an interactive `claude` session to investigate. See [docs/cli.md](./docs/cli.md#watch).

## What it does not do

- Send `repository_dispatch` events. It reads `workflow_dispatch` triggers only, so use gh-dispatch for the other kind
- Dispatch from a script. The `export` commands only read, so use `gh workflow run` in CI
- Run Actions locally. That is what act is for
- Edit or create workflow files. It reads them and dispatches them
- Work outside a repository. It discovers workflows from the checkout you are standing in

Full docs: [./docs](./docs)

## License

MIT
