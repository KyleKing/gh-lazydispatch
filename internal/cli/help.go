package cli

import (
	"slices"
	"strings"
)

// commandHelp is everything the help text says about one command. The overall
// usage and `<command> --help` both read this table, so a flag is described once.
type commandHelp struct {
	name  string
	flags string
	lines []string
}

const (
	helpHeader = `gh-lazydispatch export - read GitHub Actions without reading its logs
gh-lazydispatch watch   - wait out a run and keep its failure digest local`

	helpFooter = `export writes JSON to stdout and counts to stderr; nothing it does dispatches.
watch writes a markdown digest to disk and blocks until the run finishes.
Run "gh-lazydispatch export <command> --help" for one command's flags.`
)

const (
	flagsRuns = `Flags (runs):
  --workflow <file>   Filename, e.g. ci.yml
  --branch <name>
  --status <status>   queued|in_progress|completed|success|failure
  --event <event>     push|pull_request|merge_group|schedule
  --limit <n>         Default 20
  --current           One row per workflow: the branch's state rather than its
                      history. Needs --branch, and ignores --status and --limit`

	flagsLogs = `Flags (logs):
  --errors-only       Keep only lines that read as errors
  --step <n>          One step, by index
  --grep <pattern>    Keep lines matching a regular expression
  --limit <n>         Cap lines per step
  --format json|md    Default json`

	flagsDiagnose = `Flags (diagnose):
  --tail <n>          Lines kept from each failed step. Default 20, 0 for signatures only

Each failed step carries one "summary" line: "step (duration): cause". The cause is
the first error-level line of that step's log, with the echoed script skipped, cut to
160 characters. A step with no such line says so, or says why it has no log. The first
error can be a progress line that merely reads as an error (a test runner narrating
passes), so "errors" and "tail" still hold the lines nearest the failure. Rejected:
the last error line (it is what "tail" already anchors on), a cause classified by
pattern ("signatures" already does that and needs maintaining), and a model summary.`

	flagsJobFailures = `Flags (job-failures):
  --workflow <file>   Filename, e.g. post-deploy-smoke.yml. Required
  --runs <n>          Newest finished runs to read, every one, no sampling. Default 100
  --branch <name>
  --event <event>     push|pull_request|merge_group|schedule|workflow_dispatch

Per job: runs finished, runs failed, failed share, the step that failed most often, and
the first error-level line the newest such failure logged in that step. Canceled runs,
and jobs that were canceled or skipped, are not counted. Reads one job list per run
(cached on disk, so a rerun only reads new runs) and one log per failing job.`

	flagsCiTiming = `Flags (ci-timing):
  --branch <name>     Default: the repository's default branch
  --event <event>     Default push
  --window-days <n>   Days of commits per compared window. Default 7
  --baseline-offset-days <n>
                      Days before now the baseline window ends. Default 7
  --split <time>      RFC 3339 time or YYYY-MM-DD: compare the time since it with an equal span
                      before it, in place of the two windows above
  --until <time>      With --split, end the current window here instead of now; the baseline
                      is as long as split to until
  --baseline-until <time>
                      With --split, end the baseline here (before the split), leaving a gap
  --format json|md    json: raw rows and summaries. md: P50/P90 comparison table
  --workflow <name>   Add job queue, runner, and step timings for one workflow
  --sample <n>        With --workflow, runs per window to read jobs for. Default 150, 0 for all
  --epoch-days <n>    Split the last n days at each workflow change (and .github/ci-epochs dates); 0 skips
  --since <time>      With --epoch-days, start the range at a time instead of n days back
  --split-at <a,b>    Cut epochs at these commits or dates only, to compare before and after a merge
  --ignore-workflow <a,b>
                      Workflows that never gate a push (bots, labels): out of wall clock and epoch cuts
  --flakes            With --epoch-days, count same-SHA flakes (a job that failed in one attempt and
                      passed in a later one on the same SHA); reads every re-run attempt's jobs
  --flake-target <p>  With --flakes, the most head SHAs in percent with a flake. Default 0.5
  --author <a,b>      Compare these GitHub logins (reads only their runs); use with --event pull_request
  --epoch-min <n>     Fewest counted commits an epoch needs before it stands alone. Default 30
  --critical-sample <n>
                      With --epoch-days, read the jobs of n pushes per epoch and report where
                      the wall clock goes, which jobs end the push, runner queue, and cache steps
  --cache-logs <n>    With --critical-sample, job logs to read per job with a cache step
  --ignore-job <a,b>  With --critical-sample, job names left out of the path, such as a gate job
                      that only waits on the others
  --sample-prs <n>    Spot-check each author's n largest pull requests; --pr <n,...> names them`

	flagsCiReport = `Flags (ci-report):
  --days <n>          Days of history. Default 30
  --since <time>      Start of the range (RFC 3339 or YYYY-MM-DD), in place of --days
  --until <time>      End of the range. Default now
  --split-at <a,b>    Cut the range at these commits or dates in place of the automatic epochs,
                      so the Targets block shows each side of a merge
  --ignore-workflow <a,b>
                      Workflows that never gate a push (bots, labels): out of wall clock and epoch cuts
  --target-wall <d>   Wall clock per pull request push. Default 5m
  --target-flake <p>  Head SHAs in percent with a same-SHA flake. Default 0.5
  --target-queue <d>  Merge queue wait. Default 2m
  --deploy-workflow <a,b>
                      Deploy workflow files: merge-to-up, up, up-to-smoke, and smoke timings
  --smoke-workflow <f>
                      With --deploy-workflow, the workflow file that verifies a deploy
  --post-deploy-workflow <a,b>
                      Workflow files counted per deployed SHA. Default the smoke workflow
  --deploy-up <text>  Part of the job or step name that does the deploy. Default the whole run
  --author <a,b>      Limit pull request runs to these logins
  --window-days <n>   Days in each compared window. Default 7
  --sample <n>        Pushes per day whose jobs are read. Default 6
  --epoch-min <n>     Fewest counted commits per epoch. Default 120
  --cache-logs <n>    Job logs read per job with a cache step. Default 8, 0 skips
  --ignore-job <a,b>  Job names left out of the path and the flake count. Default: detect gate jobs
  --out <path>        Write to a file instead of stdout`

	flagsTelemetry = `Flags (telemetry):
  --format json|md    Default json`

	flagsWatch = `Flags (watch):
  --pr <n>            Watch a pull request's checks instead of one run
  --notify-cmd <cmd>  Run cmd with the outcome as its argument, after the terminal bell
  --interval <secs>   Seconds between polls. Default 15
  --out <path>        Where to write the digest. Default ./lazydispatch-run-<id>.md
  --fix               On failure, hand the digest to an interactive Claude Code
                      session to investigate. It may commit locally; it never pushes
  --fix-cmd <cmd>      The command --fix spawns. Default "claude"`
)

func helpTable() []commandHelp {
	return []commandHelp{
		{name: "workflows", lines: []string{"export workflows           Dispatchable workflows and their inputs"}},
		{name: "chains", lines: []string{"export chains              Chains defined in .github/lazydispatch.yml"}},
		{
			name:  "runs",
			lines: []string{"export runs [flags]        Recent workflow runs, newest first"},
			flags: flagsRuns,
		},
		{
			name:  "logs",
			lines: []string{"export logs <run-id>       One run's logs, parsed into steps"},
			flags: flagsLogs,
		},
		{
			name:  "diagnose",
			lines: []string{"export diagnose <run-id>   Why a run failed, without its logs"},
			flags: flagsDiagnose,
		},
		{
			name:  "job-failures",
			lines: []string{"export job-failures [flags]   Failure rate of each job across a workflow's last runs"},
			flags: flagsJobFailures,
		},
		{
			name:  "ci-timing",
			lines: []string{"export ci-timing [flags]   How long all of CI takes per commit"},
			flags: flagsCiTiming,
		},
		{
			name:  "ci-report",
			lines: []string{"export ci-report [flags]   One shareable report: pull requests, branch, merge queue"},
			flags: flagsCiReport,
		},
		{
			name:  "telemetry",
			lines: []string{"export telemetry [flags]   How often each command, verb, and dispatch was used"},
			flags: flagsTelemetry,
		},
		{
			name: cmdWatch,
			lines: []string{
				"watch <run-id> [flags]     Poll a run to completion and write its digest",
				"watch --pr <n> [flags]     Wait for a pull request's checks to settle, digest the failing runs",
			},
			flags: flagsWatch,
		},
	}
}

func (c commandHelp) render() string {
	return "gh-lazydispatch " + strings.Join(c.lines, "\n  gh-lazydispatch ")
}

// usageText is the overall usage: every command and every flag.
func usageText() string {
	table := helpTable()
	lines := make([]string, 0, len(table))
	flags := make([]string, 0, len(table))

	for _, c := range table {
		lines = append(lines, c.render())

		if c.flags != "" {
			flags = append(flags, c.flags)
		}
	}

	return helpHeader + "\n\nUsage:\n  " + strings.Join(lines, "\n  ") + "\n\n" +
		strings.Join(flags, "\n\n") + "\n\n" + helpFooter
}

// commandHelpText is one command's usage lines and flags, short enough for a screen.
func commandHelpText(name string) string {
	for _, c := range helpTable() {
		if c.name != name {
			continue
		}

		text := "Usage:\n  " + c.render()
		if c.flags != "" {
			text += "\n\n" + c.flags
		}

		return text
	}

	return usageText()
}

// helpTarget reports whether args ask for help and which command's. An empty
// name, from a missing or unknown command, asks for the overall usage.
func helpTarget(args []string) (string, bool) {
	if !slices.ContainsFunc(args[1:], func(a string) bool { return a == "-h" || a == "--help" || a == "-help" }) {
		return "", false
	}

	if args[0] == cmdWatch {
		return cmdWatch, true
	}

	return args[1], true
}
