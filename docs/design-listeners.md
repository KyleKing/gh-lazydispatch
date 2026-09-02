# Listeners over chains

A proposal. Item 1 below has shipped; the rest has not.
[docs/chains.md](./chains.md) describes what exists today.

## What the three complaints share

A chain step dispatches a workflow and then waits on the run it just created.
`ChainExecutor.runStep` calls `e.dispatch(...)` and hands the run ID it gets
back to `waitForRun`, so the only run a chain can ever wait on is one this
process started seconds ago. Wanting to deploy after the build that is
*already running* on `main` is not a missing option in the chain DSL. It is a
different question than the one the executor is built to answer.

## What a chain actually is

Three separable things, fused:

| Part   | Today                                       |
| ------ | ------------------------------------------- |
| Source | always a fresh `gh workflow run`            |
| Wait   | poll that one run until `wait_for` is met   |
| Action | dispatch the next step                      |

Every question worth asking is some other combination of those three. "Wait
for the run already going, then deploy" changes the source. "Tell me when CI
on this PR settles, with the errors" changes the action. "Merge when checks
and reviews clear" changes both.

## What GitHub already does better than a chain

Durable automation belongs in the repository, where it runs for everyone
whether or not a laptop is awake:

- [`workflow_run`](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#workflow_run)
  fires one workflow when another completes, which is exactly a two-step chain
  with `wait_for: success`
- `needs:` inside one workflow, and
  [reusable workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows)
  across several, cover the sequencing a chain expresses
- Deployment environments with protection rules gate a deploy on a human, which
  a chain cannot do at all
- `gh pr merge --auto` and the merge queue hold a merge server-side until
  required checks pass

A chain competes with those and loses on every axis that matters for
automation: it runs only while the TUI is open, only for the person who pressed
the key, and it leaves no record in the repository. What it has that they do
not is a human at the keyboard right now, holding context about one change.
That is the thing worth building for.

## The primitive

`when <condition> then <action>`, over state this tool already reads.

Conditions, each backed by code that exists:

- a named run reaches a terminal state (`github.GetWorkflowRun`, already polled
  by `internal/watcher`)
- every check on a pull request settles (`SearchPRsInRepo` rollups, already
  behind the Runs tab's PR scopes)
- a pull request becomes mergeable with no requested reviewers and no changes
  requested (one `gh pr view --json`)

Actions, likewise:

- dispatch a workflow (`internal/runner`)
- re-run failed jobs, or cancel (Phase 9 verbs)
- report the failure digest (`logs.Detect` plus `logs.ExportAsMarkdown`)
- notify, which is the one genuinely new piece and the reason the rest is worth
  anything: a watcher you have to watch has not saved you a thing
- merge, only where the invocation asked for it

A chain is then a seed plus rules: dispatch A, and register "when A succeeds,
dispatch B". Manual `C`-then-`enter` execution survives as the seed. Nothing
the chain YAML expresses today is lost, and the two cases it cannot express
come for free.

## Attaching to a run instead of starting one

The narrow fix for the case in hand. A step gains a source:

```yaml
steps:
  - workflow: docker-build.yml
    source: existing # existing | dispatch (default)
    wait_for: success
  - workflow: deploy-prod.yml
    inputs: { command: up }
```

`existing` resolves to the newest queued or in-progress run of that workflow on
the ref, from `actions/workflows/<id>/runs?branch=<ref>&status=in_progress`
plus the same for `queued`. Where nothing is running, the step fails and says
so. It does not quietly dispatch instead: a step that sometimes starts a
production build and sometimes adopts one is a step nobody can read.

The confirmation modal has to name the run it adopts, with its ID, its age, and
its URL. A modal that says `docker-build.yml (wait: success)` over a run it did
not start reads as a dispatch, which is the one misreading that costs Actions
minutes and a duplicate deploy.

## Replacing pr-merge-watch

The shell function in `~/.config/my_config/_git.sh` waits for checks to appear,
watches them with `gh pr checks --watch --fail-fast`, sleeps 60 seconds for
async reviewers, inspects reviews, then squash-merges and prunes. Two parts of
it are worse than what this tool already holds. `--fail-fast` reports that
something failed, where `logs.Detect` names the line a human would act on. The
60-second sleep is a guess at a condition that can be read directly, which is
that `reviewRequests` is empty.

That flow is a blocking terminal command inside `git push && ...`, so the
listener engine cannot live in the TUI. It belongs in a package that both a
`gh lazydispatch watch` subcommand and the TUI drive, alongside the export and
diagnose subcommands `internal/cli` already has.

## Mechanics

Polling stays. Conditional requests make it cheap: GitHub returns an `ETag`,
and a request carrying `If-None-Match` that answers `304 Not Modified` does not
count against the hourly limit, so a five-second poll on two or three
resources costs almost nothing against a PAT's 5,000 per hour. `gh run watch`
polls for the same reason.

[Webhook forwarding](https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/using-the-github-cli-to-forward-webhooks-for-testing)
(`gh webhook forward --events workflow_run`) is real, generally available, and
tempting as the foundation. It needs `admin:repo_hook`, works only on
repositories you administer, drops every event that arrives while the process
is down, and adds an HTTP listener to a terminal application. It is a later
opt-in for a repository you own, not the base case.

A listener lives in one process and dies with the terminal. The UI has to say
that rather than implying durability, because the alternative is somebody
closing a laptop and expecting a deploy.

## Safety

Arming a listener pre-authorizes a mutation that fires when nobody is looking,
which is a stronger thing than confirming a dispatch you are watching. So:
show the exact `gh` command at arm time the way the dispatch confirmation
already does, cap the window and report the expiry rather than going quiet, key
a listener on its condition and action so a re-arm cannot double-fire, and
require merge to be asked for per invocation rather than configured once.

## Order of work

1. `source: existing` on a chain step, and a confirmation modal that names the
   adopted run (shipped). Unblocks the case in hand without touching the model
2. The listener package and `gh lazydispatch watch`, with the run-completion
   and pr-checks conditions and the notify and diagnose actions. This is the
   `pr-merge-watch` replacement
3. Arming a listener from a Runs or Live row through the `a` menu, with the
   armed count in the status bar
4. Reimplement `ChainExecutor` as a seed plus listeners, and stop adding to the
   chain DSL

The failure mode to avoid is a condition language. Argo Events is worth reading
here: a filtered sensor over a webhook source covers ordinary CI, and every
attempt to grow the trigger into a workflow engine ends by recommending a real
one. Three conditions and five actions, chosen from what the tool already
reads, is the whole of it.

## Open question

Do chains stay a user-facing concept? Keeping the YAML file costs nothing and
`deploy-prod-after-build` is a name worth having. Growing the DSL costs a
second scheduler to maintain against `workflow_run`, which does the durable
version better.
