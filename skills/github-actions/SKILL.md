---
name: github-actions
description: Read GitHub Actions runs, workflows, and failure causes without pulling raw logs into context. Use when a CI run failed, when you need a workflow's dispatch inputs, or when you are about to run `gh run view --log`.
---

# Reading GitHub Actions without reading its logs

`gh run view --log` returns the whole log. A failing CI run in a Go repo is
routinely 200KB to several megabytes, and `--log-failed` still returns every
line of every failed job. Most of it is `actions/checkout` narrating git
plumbing.

`gh-lazydispatch export` parses that log once and returns the part that
answers the question. On a real CI failure in this project the same
information cost 237,144 bytes through `gh run view --log-failed` and 2,361
bytes through `export diagnose`, a factor of 100.

## Start here

Never open with `gh run view --log`. Open with:

```sh
gh-lazydispatch export diagnose <run-id>
```

That returns JSON: the run's conclusion, every step that did not succeed, the
error lines that step logged, a window of context ending at the failure, and
any known failure signature it matched with what to try about it. Read that
first and only widen if it does not explain the failure.

Widen in this order, because each step costs more than the last:

1. `export diagnose <run-id>` — the failure, a few kilobytes
2. `export logs <run-id> --errors-only` — every error line in the run
3. `export logs <run-id> --step <n>` — one step in full
4. `export logs <run-id> --grep '<pattern>'` — a hypothesis you already have
5. `gh run view --log` — only when the parse itself is what you doubt

Every command writes JSON to stdout and a line to stderr saying how many log
lines it read and how many it emitted, so the cost is visible rather than
assumed.

The cost that is not in the token count is time. `diagnose` downloads the run's
whole log before parsing it, which takes a second or two for a small run and
about nine for a 35,000-line one. Ask for one run at a time rather than fanning
out over a list.

## Commands

| Command | Answers |
| --- | --- |
| `export diagnose <run-id>` | Why did this run fail? |
| `export logs <run-id>` | What did this run print? |
| `export runs` | What ran recently, and did it pass? |
| `export workflows` | What can I dispatch, and what inputs does it take? |
| `export chains` | What multi-workflow sequences are defined here? |

Nothing under `export` dispatches anything. To actually run a workflow, use
`gh workflow run`, or `gh-lazydispatch` with no arguments for the TUI.

### diagnose

```sh
gh-lazydispatch export diagnose 33423560774
gh-lazydispatch export diagnose 33423560774 --tail 0    # signatures only
gh-lazydispatch export diagnose 33423560774 --tail 60   # more context
```

`--tail` sets how many lines of context to keep before the failure, defaulting
to 20. The window ends at the last error line rather than at the end of the
step, because a job's final lines are its teardown.

`errors` holds the last twenty error-classified lines of the step, so a test
runner that narrates thousands of passing tests before it fails reports the
summary naming the cause rather than the narration.

`signatures` names failure modes it recognizes (out of memory, out of disk,
timeout, missing secret, permission denied, network failure) with the line that
matched and a first thing to try. It reads only a step that failed, and only
that step's last 200 lines, because a signature is a regular expression over log
text and log text is prose: further back in a long step it matches a
parametrized test ID reading `[access denied not retryable]` or a story titled
`Missing Secret Error`. Read the quoted line before acting on the label, and
treat an empty `signatures` on a genuine failure as normal rather than as a
parse problem. Most failures are not one of six known modes, which is what
`failed_steps` is for.

### logs

```sh
gh-lazydispatch export logs 33423560774 --errors-only
gh-lazydispatch export logs 33423560774 --step 3 --limit 200
gh-lazydispatch export logs 33423560774 --grep 'panic|FAIL' --format md
```

Flags work before or after the run ID. `--format md` renders a document to
paste into an issue. `--limit` caps lines per step and reports how many it
dropped.

`--errors-only` and the signature scan both skip the block GitHub folds into
the top of every `run:` step, which holds the script's own source. A script
that says `echo "Error: ..."` is not a step that failed.

### runs, workflows, chains

```sh
gh-lazydispatch export runs --workflow ci.yml --branch main --limit 5
gh-lazydispatch export runs --status failure
gh-lazydispatch export workflows
gh-lazydispatch export chains
```

`export workflows` reads `.github/workflows/` in the working directory and
returns each dispatchable workflow with its inputs, their types, defaults, and
permitted values. Read that instead of opening the YAML files when you need to
know what a workflow accepts.

`export runs` takes a workflow filename, not its display name.

## Installing

```sh
gh extension install kyleking/gh-lazydispatch
```

That installs it as `gh lazydispatch`; the export commands work the same way
(`gh lazydispatch export diagnose <run-id>`). A standalone binary from the
releases page is invoked as `gh-lazydispatch`.

Both need `gh` on PATH and authenticated, and the working directory must be
inside the repository you are asking about, or `GH_REPO` set to `owner/repo`.
