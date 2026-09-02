# gh-lazydispatch specifics

What [AGENTS.md](AGENTS.md) does not cover because it is template-owned and this is not.

## Recorded gh interactions

Everything that reaches GitHub goes through the `gh` binary: every run, job, and log
read is `exec.CommandExecutor` shelling out to it. So the recording seam is `PATH`, not
an HTTP transport. `aragonite/ghcassette` builds a stand-in `gh`, puts it ahead of the
real one, and either records what the real binary did or answers from a cassette.

The point is the shape, not the convenience. `gh run view --log` prefixes every line
with `job<TAB>step<TAB>timestamp message`, stores GitHub's coloring as caret notation
rather than escape bytes, and opens the first line with a byte order mark. A fixture
written by hand records what its author believed gh emits, so a parser built on the
same belief passes every test and finds nothing in production. That is exactly what
happened: `parseJobLogsIntoSteps` split on `##[group]` at the start of a line, no
fixture disagreed, and the log viewer was empty for every real run.

A cassette is TOML named `.golden`, because `hk.pkl` excludes that suffix from the
whitespace fixers and nothing else. A recorded log whose trailing spaces and tabs were
normalized on commit is no longer what gh printed.

Four packages hold cassettes, each covering the layer above the last:

- `internal/logs` has `passing-run` and `failing-run`, the step-log parser and the
  failure signatures against real log bytes. `TestRecorded_FixturesMatchTheRecordedShape`
  holds every fixture under `testdata/logs/` to the same line shape, so a new one cannot
  be written in a format gh never emits
- `internal/cli` has the export subcommands, driven as a built binary
- `internal/github` has `reads`, `pull-requests`, and `environments`: every read method
  on the client, against the JSON GitHub actually returns
- `internal/app` has `runs-journey`, the path the tool exists for, in one recording: a
  branch's state, the run behind a row, and that run's log. It is one test rather than
  four because the branch listing alone is a megabyte of JSON, and a second test wanting
  a second copy of it is worth restructuring to avoid

A recording is only worth its bytes if what it recorded is not empty. Both cassette
tests that could go vacuous fail instead: the pull request scopes require a rollup to
have parsed, and the branch state requires a row. `KyleKing/second-look` is the pull
request target because this repository has no open pull request of its own, and a
recording of an empty list tests nothing.

A pull request listing resolves its repository from the *working directory's* remote
rather than from the repository the client was built for, so the test stands the process
in a throwaway checkout whose only remote is the target. That coupling is a defect
rather than a convention; ROADMAP.md carries it.

### Re-recording

```sh
mise run test:record
```

This reads run history through the real `gh` and rewrites the cassettes. It creates
nothing: the runs it reads are already finished, and re-recording never dispatches.
Recording a *dispatch* is what `mise run test:live` is for, and that one spends Actions
minutes; see [docs/live-testing.md](docs/live-testing.md).

The run IDs are constants in each package's `recorded_internal_test.go`. They belong to
`demo-test.yml` and `demo-chain-check.yml` on `main`, which exist to be dispatched and
are never removed. Pick new IDs there if GitHub ages these out; `demo-chain-check.yml`
prints a line matching every signature `logs.Detect` looks for, which is what the export
test asserts on.

Recording talks to GitHub as you. `internal/exec`'s mutation guard panics on
`gh workflow run` and friends during tests, so an in-process test cannot dispatch by
accident, and replay reaches nothing at all: the stub has no `gh` to run. The guard
allowlists read *operations* (`list`, `view`, `status`, `checks`, `diff`, `watch`)
against the subcommands that can write, so `gh pr list` gets through and `gh pr create`
does not. Anything unrecognized stays blocked, which is why a gh release adding an
operation blocks until someone decides it reads.

A recording is committed verbatim to a public repository, so
`testrepo.RequirePublic` refuses to record anything that is not public. It runs before
`ghcassette.Start`, whose first act when recording is to delete the cassette: a guard
placed after it has already destroyed what it was protecting. It is inert on replay.

Three more things a cassette test has to do, each of which passes silently when skipped:

- Call `cache.ClearAll()` before replaying. aragonite caches some reads in process, and
  a cached read never reaches the stub, so the replay passes while playing nothing
- Resolve the cassette path before any `t.Chdir`, or `testdata` resolves against
  wherever the test went
- Let `ghcassette.Start` run before any `t.Chdir` too. It builds the stand-in with
  `go build`, which needs the module. The stub is built once per test process, so a test
  that gets this wrong passes whenever another test ran first and fails alone

## Async messages and the modal stack

`Update` routes background messages to their handlers before the modal stack sees them.
Routing by "is a modal open" is what stopped a chain's status modal ever seeing its own
updates, since it is the active modal for the whole run, and what dropped a modal's
result when a keystroke raced it. `TestUpdate_AsyncMessagesReachTheirHandlersBehindAModal`
pins that ordering.

## Failure signatures and the corpus that tuned them

`logs.Detect` reads only a step that failed, and only that step's last
`logs.SignatureWindow` lines. Both numbers came from measurement, not judgment, so change
them the same way.

The demo workflows cannot answer the question these rules settle. `demo-chain-check.yml`
prints a line matching every signature on purpose, which proves the regexes fire and says
nothing about whether they fire on the wrong line. That needs a repository whose logs
nobody wrote for this tool.

To re-measure, take a real repository with many workflows, download `gh run view --log`
for a stratified sample of its failed *and* successful runs, group each capture's lines by
the `job<TAB>step` prefix, and run the candidate rule over the groups. Then read every
match and label it by hand: the only thing that matters is whether the match names the
cause a human would act on.

The last pass covered 42 runs of a 26-workflow repository and found 12 matches, 2 of them
real. What the false ones had in common is that log text is prose: a parametrized test ID
reads `[access denied not retryable]`, a Storybook story is titled `Missing Secret Error`,
and a service logs `SECRET is not set - using an ephemeral random secret` on a healthy
run. None of that is distinguishable from a cause by any regex worth maintaining, which is
why the rules are about *where* a match sits rather than what it says.

Two conclusions worth keeping, because both were tried and rejected:

- **Do not require a `##[error]` marker.** One of the two real causes was a `##[notice]`
  carrying `HTTP/1.1 403 Forbidden`, and its run logged no `##[error]` at all
- **Do not tie the window to `--tail`.** The other real cause sat 329 lines from its
  step's end, well outside the 20-line excerpt, so the excerpt and the scan need separate
  windows
