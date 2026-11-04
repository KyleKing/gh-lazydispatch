package citiming

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

// DeployConfig names the workflows of a deploy pipeline, as workflow file names.
// Nothing here is specific to one repository.
type DeployConfig struct {
	// Smoke is the workflow that verifies a deploy.
	Smoke string
	// UpMarker is a case-insensitive part of the job or step name that does the deploy.
	// Empty measures the whole deploy run.
	UpMarker string
	// Workflows are the deploy workflows, one table each.
	Workflows []string
	// PostDeploy are the workflows whose runs on a deployed SHA are counted per deploy.
	PostDeploy []string
}

// Enabled reports whether any deploy workflow is configured.
func (c DeployConfig) Enabled() bool { return len(c.Workflows) > 0 }

// ciSpanStat is a duration's count and percentiles in seconds.
type ciSpanStat struct {
	N          int     `json:"n"`
	P50Seconds float64 `json:"p50_seconds"`
	P90Seconds float64 `json:"p90_seconds"`
}

// ciPostDeploy is one post-deploy workflow's runs on the deployed SHAs.
type ciPostDeploy struct {
	Workflow  string  `json:"workflow"`
	Runs      int     `json:"runs"`
	PerDeploy float64 `json:"per_deploy"`
	// Repeated counts deploys on which this workflow ran more than once.
	Repeated int `json:"repeated_deploys"`
}

// ciDeploy is one deploy workflow's pipeline timings.
type ciDeploy struct {
	Workflow   string         `json:"workflow"`
	PostDeploy []ciPostDeploy `json:"post_deploy"`
	MergeToUp  ciSpanStat     `json:"merge_to_up_start"`
	Up         ciSpanStat     `json:"up"`
	UpToSmoke  ciSpanStat     `json:"up_end_to_smoke_start"`
	Smoke      ciSpanStat     `json:"smoke"`
	Deploys    int            `json:"deploys"`
	WithoutUp  int            `json:"without_up"`
}

// notPushGated lists the events that start a workflow before a merge, which a
// post-deploy count must leave out.
var notPushGated = map[string]bool{eventMergeGroup: true, eventPullRequest: true, "pull_request_target": true}

func collectDeploys(
	client *github.Client, cfg DeployConfig, from, to time.Time, stderr io.Writer,
) ([]ciDeploy, error) {
	deployRuns := make(map[string][]github.WorkflowRunInfo, len(cfg.Workflows))

	var ids []int64

	for _, file := range cfg.Workflows {
		runs, err := client.ListWorkflowRuns(file, from, to)
		if err != nil {
			return nil, fmt.Errorf("listing deploys: %w", err)
		}

		deployRuns[file] = deploysOnly(runs)

		for i := range deployRuns[file] {
			ids = append(ids, deployRuns[file][i].ID)
		}
	}

	var smoke []github.WorkflowRunInfo

	if cfg.Smoke != "" {
		var err error
		if smoke, err = client.ListWorkflowRuns(cfg.Smoke, from, to); err != nil {
			return nil, fmt.Errorf("listing smoke runs: %w", err)
		}
	}

	post := make(map[string][]github.WorkflowRunInfo, len(cfg.PostDeploy))

	for _, file := range cfg.PostDeploy {
		runs, err := client.ListWorkflowRuns(file, from, to)
		if err != nil {
			return nil, fmt.Errorf("listing post-deploy runs: %w", err)
		}

		post[file] = runs
	}

	jobs := runcache.Jobs{}

	if cfg.UpMarker != "" {
		var err error
		if jobs, err = loadJobs(client, ids, stderr); err != nil {
			return nil, err
		}
	}

	out := make([]ciDeploy, 0, len(cfg.Workflows))
	for _, file := range cfg.Workflows {
		out = append(out, summarizeDeploys(file, deployRuns[file], jobs, smoke, post, cfg.UpMarker, to))
	}

	return out, nil
}

// deploysOnly keeps the runs that reached a pass or fail, oldest first.
func deploysOnly(runs []github.WorkflowRunInfo) []github.WorkflowRunInfo {
	var out []github.WorkflowRunInfo

	for i := range runs {
		if verdict(runs[i].Conclusion) {
			out = append(out, runs[i])
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })

	return out
}

// upWindow finds when the deploy itself ran: the first job or step whose name
// holds marker, or the whole run when marker is empty.
func upWindow(run *github.WorkflowRunInfo, jobs []github.JobTiming, marker string) (time.Time, time.Time, bool) {
	if marker == "" {
		start := run.RunStartedAt
		if start.IsZero() {
			start = run.CreatedAt
		}

		return start, run.UpdatedAt, true
	}

	marker = strings.ToLower(marker)

	for i := range jobs {
		if strings.Contains(strings.ToLower(jobs[i].Name), marker) && !jobs[i].StartedAt.IsZero() &&
			!jobs[i].CompletedAt.IsZero() {
			return jobs[i].StartedAt, jobs[i].CompletedAt, true
		}
	}

	for i := range jobs {
		for _, step := range jobs[i].Steps {
			if strings.Contains(strings.ToLower(step.Name), marker) && !step.StartedAt.IsZero() &&
				!step.CompletedAt.IsZero() {
				return step.StartedAt, step.CompletedAt, true
			}
		}
	}

	return time.Time{}, time.Time{}, false
}

type deploySamples struct {
	postRuns                        map[string][]int
	mergeToUp, up, upToSmoke, smoke []float64
}

// summarizeDeploys measures each deploy of one workflow. A smoke run belongs to
// the deploy when it ran on the deployed SHA after the up, or else when it is
// the first one to start after the up and before the next deploy. A post-deploy
// run counts when it ran on the deployed SHA between this deploy and the next.
func summarizeDeploys(
	workflow string, deploys []github.WorkflowRunInfo, jobs runcache.Jobs, smoke []github.WorkflowRunInfo,
	post map[string][]github.WorkflowRunInfo, marker string, rangeEnd time.Time,
) ciDeploy {
	out := ciDeploy{Workflow: workflow, Deploys: len(deploys)}
	samples := deploySamples{postRuns: make(map[string][]int)}

	for i := range deploys {
		d := &deploys[i]

		next := rangeEnd
		if i+1 < len(deploys) {
			next = deploys[i+1].CreatedAt
		}

		start, end, ok := upWindow(d, jobs[d.ID], marker)
		if !ok {
			out.WithoutUp++

			continue
		}

		if !d.CommittedAt.IsZero() {
			samples.mergeToUp = append(samples.mergeToUp, nonNegative(start.Sub(d.CommittedAt)))
		}

		samples.up = append(samples.up, end.Sub(start).Seconds())

		if s := smokeFor(d, end, next, smoke); s != nil {
			samples.upToSmoke = append(samples.upToSmoke, nonNegative(runStart(s).Sub(end)))
			samples.smoke = append(samples.smoke, s.UpdatedAt.Sub(runStart(s)).Seconds())
		}

		for file, runs := range post {
			samples.postRuns[file] = append(samples.postRuns[file], countOnSHA(runs, d, next))
		}
	}

	out.MergeToUp, out.Up = spanStat(samples.mergeToUp), spanStat(samples.up)
	out.UpToSmoke, out.Smoke = spanStat(samples.upToSmoke), spanStat(samples.smoke)
	out.PostDeploy = postDeployRows(samples.postRuns)

	return out
}

func runStart(run *github.WorkflowRunInfo) time.Time {
	if run.RunStartedAt.IsZero() {
		return run.CreatedAt
	}

	return run.RunStartedAt
}

func smokeFor(
	d *github.WorkflowRunInfo, upEnd, next time.Time, smoke []github.WorkflowRunInfo,
) *github.WorkflowRunInfo {
	var sameSHA, firstAfter *github.WorkflowRunInfo

	for i := range smoke {
		s := &smoke[i]
		if !verdict(s.Conclusion) || s.CreatedAt.Before(d.CreatedAt) || !s.CreatedAt.Before(next) {
			continue
		}

		if s.HeadSHA == d.HeadSHA && (sameSHA == nil || s.CreatedAt.Before(sameSHA.CreatedAt)) {
			sameSHA = s
		}

		if !s.CreatedAt.Before(upEnd) && (firstAfter == nil || s.CreatedAt.Before(firstAfter.CreatedAt)) {
			firstAfter = s
		}
	}

	if sameSHA != nil {
		return sameSHA
	}

	return firstAfter
}

func countOnSHA(runs []github.WorkflowRunInfo, d *github.WorkflowRunInfo, next time.Time) int {
	n := 0

	for i := range runs {
		r := &runs[i]
		if r.HeadSHA == d.HeadSHA && !notPushGated[r.Event] && !r.CreatedAt.Before(d.CreatedAt) &&
			r.CreatedAt.Before(next) {
			n++
		}
	}

	return n
}

func spanStat(seconds []float64) ciSpanStat {
	return ciSpanStat{
		N:          len(seconds),
		P50Seconds: percentile(seconds, percentileMedian),
		P90Seconds: percentile(seconds, percentileP90),
	}
}

func postDeployRows(perDeploy map[string][]int) []ciPostDeploy {
	out := make([]ciPostDeploy, 0, len(perDeploy))

	for file, counts := range perDeploy {
		row := ciPostDeploy{Workflow: file}
		for _, n := range counts {
			row.Runs += n
			if n > 1 {
				row.Repeated++
			}
		}

		if len(counts) > 0 {
			row.PerDeploy = float64(row.Runs) / float64(len(counts))
		}

		out = append(out, row)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Workflow < out[j].Workflow })

	return out
}

func writeDeploys(b *strings.Builder, deploys []ciDeploy) {
	if len(deploys) == 0 {
		return
	}

	b.WriteString("\n## Deploys\n\nEach deploy is a finished run of a deploy workflow. Merge is the commit time of " +
		"the " +
		"deployed SHA. Up is the configured job or step. Smoke runs are matched on the deployed SHA, or else the " +
		"first one to start after the up and before the next deploy. Post-deploy runs count on the deployed SHA " +
		"between this deploy and the next.\n")

	for i := range deploys {
		d := &deploys[i]

		writeTableHead(b, fmt.Sprintf("%s: %d deploys (%d without a matching up).", d.Workflow, d.Deploys, d.WithoutUp),
			"<Phase", "Deploys", "P50", "P90")

		for _, phase := range []struct {
			name string
			s    ciSpanStat
		}{
			{"Merge to up start", d.MergeToUp},
			{"Up duration", d.Up},
			{"Up end to smoke start", d.UpToSmoke},
			{"Smoke duration", d.Smoke},
		} {
			fmt.Fprintf(b, "| %s | %d | %s | %s |\n", phase.name, phase.s.N, formatSeconds(phase.s.P50Seconds),
				formatSeconds(phase.s.P90Seconds))
		}

		if len(d.PostDeploy) > 0 {
			writeTableHead(b, "Post-deploy runs per deploy. More than one run of a workflow on a SHA is redundant.",
				"<Workflow", "Runs", "Per deploy", "Deploys with more than one")

			for _, p := range d.PostDeploy {
				fmt.Fprintf(b, "| %s | %d | %.2f | %d |\n", p.Workflow, p.Runs, p.PerDeploy, p.Repeated)
			}
		}
	}
}
