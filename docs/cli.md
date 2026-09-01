# The export commands

`gh-lazydispatch export` is the non-interactive half of the tool. It reads the
same GitHub Actions data the TUI reads, through the same packages, and writes
JSON to stdout. Nothing under `export` dispatches: to run a workflow, use the
TUI or `gh workflow run`.

It exists because `gh run view --log` is the wrong shape for a question like
"why did this fail". The log is the whole log, `--log-failed` is every line of
every failed job, and most of both is `actions/checkout` narrating git
plumbing. `export diagnose` parses that once and returns the failure.

Measured against run
[33423560774](https://github.com/KyleKing/gh-lazydispatch/actions/runs/33423560774),
a CI failure in this repository:

| Command | Bytes |
| --- | --- |
| `gh run view --log` | 408,254 |
| `gh run view --log-failed` | 237,144 |
| `gh-lazydispatch export diagnose` | 2,361 |
| `gh-lazydispatch export logs --errors-only` | 362 |

The saving scales with the log, so it is largest exactly when reading the log
by hand is worst. On a nine-line demo workflow the same commands are within a
factor of two of each other.

Every command writes its line counts to stderr (`read 4088 lines, emitted 14`),
so what a filter saved is visible rather than assumed.

## diagnose

```sh
gh-lazydispatch export diagnose <run-id> [--tail <n>]
```

Returns the run's conclusion and URL, every step that did not succeed, and the
failure signatures it recognized.

Each failed step carries `errors`, the lines the parser read as errors, and
`tail`, a window of context. The window ends at the last error line rather than
at the end of the step: when gh cannot resolve a job's steps the whole job
becomes one step, and its last lines are the runner's teardown rather than
anything that went wrong. `--tail 0` drops the context and keeps the errors and
signatures.

`signatures` are regular expressions over log text, so each one reports the
line that matched alongside the label. A test named "handles timeouts" matches
`Timeout`; the quoted line is what tells you it is not one.

## logs

```sh
gh-lazydispatch export logs <run-id> [--errors-only] [--step <n>] [--grep <re>]
                                     [--limit <n>] [--format json|md]
```

Returns the run's log parsed into steps, with timestamps and terminal escapes
already stripped. Flags may be written before or after the run ID.

`--format md` renders a document with a heading per step and the detected
signatures at the top, which is what a reader pastes into an issue.

`--limit` caps lines per step and reports how many it dropped in `truncated`.

`--errors-only` skips the fold GitHub writes at the top of every `run:` step,
which holds the script's own source. A script containing `echo "Error: ..."`
is not a step that failed, and the same rule governs signature detection.

## runs

```sh
gh-lazydispatch export runs [--workflow <file>] [--branch <name>]
                            [--status <status>] [--limit <n>]
```

Recent runs, newest first, reduced to the fields that decide what to do next.
`--workflow` takes a filename (`ci.yml`), not a display name.

## workflows and chains

```sh
gh-lazydispatch export workflows
gh-lazydispatch export chains
```

Neither touches the network. `workflows` reads `.github/workflows/` in the
working directory and returns every dispatchable workflow with its inputs,
their types, defaults, and permitted values. `chains` reads
`.github/lazydispatch.yml` and returns each chain's variables and steps in
order.

## Requirements

`gh` on PATH and authenticated. The working directory must be inside the
repository being asked about, or `GH_REPO` set to `owner/repo`.

## For agents

[skills/github-actions](../skills/github-actions) is the same material written
as an agent skill, with the widen-in-this-order rule that keeps a debugging
session from opening with the whole log.
