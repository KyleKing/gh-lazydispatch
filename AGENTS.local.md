# gh-lazydispatch specifics

What [AGENTS.md](AGENTS.md) does not cover because it is template-owned and this is not.

## Recorded gh interactions

Everything that reaches GitHub goes through the `gh` binary: every run, job, and log
read is `exec.CommandExecutor` shelling out to it. So the recording seam is `PATH`, not
an HTTP transport. `internal/ghcassette` builds a stand-in `gh`, puts it ahead of the
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

`internal/logs/testdata/cassettes/` holds `passing-run` and `failing-run`, recorded
against completed runs of the dispatch-only demo workflows. `TestRecorded_*` in
`internal/logs` replays them; `TestRecorded_FixturesMatchTheRecordedShape` holds every
fixture under `testdata/logs/` to the same line shape, so a new one cannot be written
in a format gh never emits.

### Re-recording

```sh
mise run test:record
```

This reads run history through the real `gh` and rewrites the cassettes. It creates
nothing: the runs it reads are already finished, and re-recording never dispatches.
Recording a *dispatch* is what `mise run test:live` is for, and that one spends Actions
minutes; see [docs/live-testing.md](docs/live-testing.md).

The run IDs are constants in `internal/logs/recorded_internal_test.go`. They belong to
`demo-test.yml` and `demo-chain-check.yml` on `main`, which exist to be dispatched and
are never removed. Pick new IDs there if GitHub ages these out; `demo-chain-check.yml`
prints a line matching every signature `logs.Detect` looks for, which is what the export
test asserts on.

Recording talks to GitHub as you. `internal/exec`'s mutation guard panics on
`gh workflow run` and friends during tests, so an in-process test cannot dispatch by
accident, and replay reaches nothing at all: the stub has no `gh` to run.

## Async messages and the modal stack

`Update` routes background messages to their handlers before the modal stack sees them.
Routing by "is a modal open" is what stopped a chain's status modal ever seeing its own
updates, since it is the active modal for the whole run, and what dropped a modal's
result when a keystroke raced it. `TestUpdate_AsyncMessagesReachTheirHandlersBehindAModal`
pins that ordering.
