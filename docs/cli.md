# The non-interactive commands

`gh-lazydispatch export` and `gh-lazydispatch watch` are the non-interactive
half of the tool. Both read the same GitHub Actions data the TUI reads,
through the same packages. Neither dispatches: to run a workflow, use the TUI
or `gh workflow run`.

`export` writes JSON to stdout.

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
`tail`, a window of context. Both end at the last error line rather than at the
end of the step: when gh cannot resolve a job's steps the whole job becomes one
step, and its last lines are the runner's teardown rather than anything that
went wrong. `errors` is capped at twenty and keeps the last of them for the same
reason. `--tail 0` drops the context and keeps the errors and signatures.

`signatures` are regular expressions over log text, so each one reports the
line that matched alongside the label. The scan reads only a step that failed,
and only its last `logs.SignatureWindow` lines, because anywhere else in a long
step the patterns match ordinary output rather than a cause. Read the quoted
line before acting on the label. `AGENTS.local.md` records how those two rules
were measured and how to re-measure them.

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
                            [--status <status>] [--limit <n>] [--current]
```

Recent runs, newest first, reduced to the fields that decide what to do next.
`--workflow` takes a filename (`ci.yml`), not a display name.

`--current` answers the other question: not what has run, but what state each
workflow is in on a branch now. It keeps the newest run of each workflow, keyed
on the workflow file and its display title together so a workflow reporting a
mode in its title keeps one state per mode, and it needs `--branch`.

## ci-timing

```sh
gh-lazydispatch export ci-timing [--branch <name>] [--event <event>]
    [--window-days <n>] [--baseline-offset-days <n>] [--format json|md]
    [--workflow <name>] [--sample <n>] [--epoch-days <n>]
    [--author <login,...>] [--sample-prs <n>] [--pr <n,...>] [--critical-sample <n>] [--ignore-job <name,...>]
```

How long all of CI takes to finish, per commit: the span from the earliest
triggered run's creation to the last one's completion, across every workflow
one push started. `--event` defaults to `push`, and `--branch` to the
repository's default branch for `push` and to any branch for other events
(`pull_request`, `merge_group`). Runs are read one day at a time, because the Actions
API returns at most 1000 results for a filtered query.

A commit stays out of every percentile (and is counted under `excluded`) when
a run is still in flight, timed out or failed to start, or was canceled after
every completed run, because its span is then cut short. A canceled run that
finishes before a completed one is kept, since a newer push usually supersedes
one workflow while the rest finish. Each commit row carries its `excluded`
reason so nothing is lost from the raw data.

The output carries every commit in the sample plus min, p50,
mean, p90, and max across the counted ones, so a repository can answer "how long do I
wait for CI to go green" without opening the Actions tab and doing the math
by hand.

`windows` in the JSON, and the whole of `--format md`, compare two spans of
`--window-days` (default 7): the baseline ending `--baseline-offset-days`
(default 7) ago and the current one ending now. A commit belongs to the window
its CI began in. The markdown table gives commits, p50, p90, and max for each
window plus the p90 change and a per-workflow table of when each workflow
finishes after the push, with how often it finishes last. A warning prints for any range that still hit the 1000-result cap.

`--workflow <name>` drills into one workflow (the one that finishes last most
often, from the per-workflow table). It reads that workflow's jobs, one API
call per run, and adds a job table (runner labels, queue wait, run time) and a
slowest-steps table for each window. Queue wait is a job's start minus its
creation, which is the time GitHub took to hand it to a runner. A runner label
that differs between the windows is shown beside the job, because a runner
swap moves every number after it. Jobs are cached by run ID in
`ci-jobs.json` beside the run cache, since a finished run's jobs never change.
Jobs of canceled runs are not read. To spare the API's rate limit
(5000 calls an hour) only `--sample` runs per window (default 150, 0 for all)
are read, picked by a hash of the run ID so the same runs are chosen as the
window slides. A rate limit hit mid-way keeps what was fetched, and a rerun
resumes.

`--epoch-days <n>` splits the last n days at each commit that touched
`.github/workflows` or `.github/actions` on `origin/<branch>` (run it from a
checkout), plus the dates in an optional `.github/ci-epochs` file (one
`YYYY-MM-DD label` per line, for changes git cannot show such as a runner
swap). The table gives each epoch's commits, p50, p90, and max. An epoch
with fewer than `--epoch-min` (default 30) counted commits folds into the next,
because a P90 over fewer is noise. Only the newest epoch, or a manual one, can
stay small, and it is marked "(few)".

A run created more than ten minutes after the push's first run (a title
edit, a label, a manual dispatch) is a retrigger: it stays in `run_details`
but out of the span. A commit with a rerun attempt is left out as `rerun`,
since the wait for someone to press the button is not CI time.

`--author a,b` reads only those logins' runs (the API filters by actor, so a
busy repository stays affordable) and adds tables by author: baseline against
current window, then epoch, then day, all cut at the same dates so the rows
line up. `--epoch-days` also adds workflow epochs, which cut each workflow's
run duration at the commits that changed that workflow's own file, to say
which change moved which workflow. Workflow epochs need the run's `path`, so a
cache written before it existed has none: delete it to rebuild. Epochs open
with a "start of range" row so the span before the first change is a row too.

`--epoch-days` also prints how pushes ended in each epoch (counted, canceled,
re-run, timed out, unfinished) and each slow workflow's run time, failure rate,
and cancel rate per epoch, from run data alone.

`--critical-sample n` reads the jobs of n counted pushes per day (chosen by
a hash, so the same pushes return as the range grows) and groups them by epoch and splits each push's
wall clock along the job that finished last: waiting on earlier jobs (the
job's creation after the push began), runner queue, the job running, and the
tail after it. It ranks the jobs that end pushes by headroom, the gap to the
job that finished before them, which is the most a push gains from speeding up
that job alone. It also tabulates queue wait by runner label across every
job, and steps whose name mentions a cache (their time and skip share, since
the API does not report hits). Job reads are cached by run ID. A gate job that
only waits on the others always finishes last, so `--ignore-job` drops it and
the path ends at the real work.

The same job sample feeds more tables. A runner comparison finds when
Blacksmith runners were in use and lines each job seen on them up against its
own runs before, during, and after. Handoff tables read the `needs` edges of
the checkout's workflows and report the lag between a job's last dependency
finishing and the job starting, plus the most common dependency chains behind
the last job to finish. The widest-spread jobs list the step that varies most
and a cache signal: how often a cache post step uploaded an entry (which
follows a miss) and the job's median with and without an upload. A spend table
gives runner minutes and billed minutes (each job rounded up to a minute) per
push, the share of jobs skipped, and the jobs skipped most. Failure and re-run
tables count a workflow's failed runs and its re-runs that ended green, the
closest the API gets to flakiness, since it cannot tell a flake from a fix
pushed in between.

With `--event merge_group`, the entries of the merge queue come from each pull
request's timeline (GraphQL): entries, merges, evictions (any removal other
than a merge) with their reasons, and the wait from joining the queue to
merging. Merge group CI time uses the same windows and epochs as any event.

`--sample-prs n` picks each author's n largest pull requests (by files
changed) from their eight most-pushed, and `--pr 1,2` adds named ones. Each
row gives the PR's size beside the P50 and P90 CI time of its pushes and the
workflow that finished last. Epoch dates come from the default branch's
history, while a PR run uses the workflow files of its own merge ref.

Finished runs are cached in
`$XDG_CACHE_HOME/lazydispatch/<owner>/<repo>/ci-timing-<event>-<branch|all>[-<actor>].json`
(`~/.cache` when unset), along with the time range they cover, so a repeat
report fetches only runs created since. A run still in flight is not stored
and is read again next time. Delete the file to rebuild it, for example after
a rerun changed an old run's result.

Each commit also carries `committed_at` and a `run_details` row per workflow
run (name, conclusion, start, finish, seconds, seconds after the push), so the same output can be
re-cut by percentile, by slowest workflow, or by gap between commit and push
without another API call. The clock starts at the first run's creation and
not at `committed_at`, because a commit made locally and pushed days later
would otherwise count the wait for the push as CI time. Runs are Actions
workflow runs; checks from other apps are not included.

## ci-report

```sh
gh-lazydispatch export ci-report [--days <n>] [--author <login,...>]
    [--window-days <n>] [--sample <n>] [--epoch-min <n>] [--ignore-job <name,...>]
    [--out <path>]
```

One markdown report for the repository's pull requests (every author, or
`--author`), pushes to the default branch, and merge queue. Each section is
`ci-timing --format md` with `--epoch-days` and `--critical-sample` set, and a
trigger with no runs in the range is left out. Above the sections is a list of
findings, each a rule over the tables: a P90 that moved more than 10% between the
windows, over 15% of pushes canceled or re-run, the job and chain that most often
end a push, billed minutes per push that grew more than 30%, jobs skipped on
every sampled push, re-runs that passed, jobs whose P90 is twice their median,
a runner vendor trial, and merge queue evictions over 5%. Thresholds are
constants in `internal/citiming/cireport.go`.

Gate jobs (a job that ends a quarter of the pushes within 15 seconds of the job
before it) are detected and left out of the path, because they only wait on
other jobs. `--ignore-job` names them instead.

Reading jobs is the cost, about 13 REST calls per sampled push. The command
checks the remaining allowance first and stops with the time it resets rather
than writing a report from part of the data. Runs and jobs are cached, so a
rerun after the reset resumes.

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

## watch

```sh
gh-lazydispatch watch <run-id> [--interval <secs>] [--out <path>]
                               [--fix] [--fix-cmd <cmd>]
```

The `pr-merge-watch` replacement `docs/design-listeners.md` describes. Chain
after a push:

```sh
git push && gh-lazydispatch watch "$(gh run list --limit 1 --json databaseId -q '.[0].databaseId')"
```

It polls the run every `--interval` seconds (15 by default) until it
completes, then writes an errors-only markdown digest (the same document
`export logs --format md --errors-only` renders) to `--out` (default
`./lazydispatch-run-<id>.md` in the working directory) and prints that path on
stdout. That alone is the point: a summary that stays on disk instead of a
terminal you have to keep open.

`--fix` runs only when the run failed. It hands the digest's path to an
interactive `claude` session in the foreground (`--fix-cmd` to use a different
one), the same way `git commit` hands the terminal to `$EDITOR`, with a prompt
naming the run and asking it to investigate, fix, and commit locally on its
own branch. It never tells it to push, and `watch` itself never touches git.

## Requirements

`gh` on PATH and authenticated. The working directory must be inside the
repository being asked about, or `GH_REPO` set to `owner/repo`.

## For agents

[skills/github-actions](../skills/github-actions) is the same material written
as an agent skill, with the widen-in-this-order rule that keeps a debugging
session from opening with the whole log.
