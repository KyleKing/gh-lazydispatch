package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	workflowGlob = ".github/workflows/*.y*ml"
	topChains    = 3
	chainArrow   = " -> "
)

// needsGraph maps a workflow's name to its jobs' display names and what each needs.
type needsGraph map[string]map[string][]string

type workflowJob struct {
	Name  string    `yaml:"name"`
	Needs yaml.Node `yaml:"needs"`
}

type workflowFile struct {
	Jobs map[string]*workflowJob `yaml:"jobs"`
	Name string                  `yaml:"name"`
}

// readNeeds parses the workflows of the checkout in the working directory.
// A workflow that fails to parse is skipped.
func readNeeds() needsGraph {
	files, err := filepath.Glob(workflowGlob)
	if err != nil {
		return nil
	}

	graph := make(needsGraph)

	for _, path := range files {
		data, err := os.ReadFile(path) //nolint:gosec // globbed inside the checkout
		if err != nil {
			continue
		}

		var wf workflowFile
		if yaml.Unmarshal(data, &wf) == nil && wf.Name != "" {
			graph[wf.Name] = wf.edges()
		}
	}

	return graph
}

// edges maps each job's display name to the display names it needs.
func (wf *workflowFile) edges() map[string][]string {
	display := make(map[string]string, len(wf.Jobs))

	for key, job := range wf.Jobs {
		display[key] = key
		if job.Name != "" {
			display[key] = job.Name
		}
	}

	jobs := make(map[string][]string, len(wf.Jobs))

	for key, job := range wf.Jobs {
		jobs[display[key]] = nil

		for _, need := range needKeys(&job.Needs) {
			if name, ok := display[need]; ok {
				jobs[display[key]] = append(jobs[display[key]], name)
			}
		}
	}

	return jobs
}

func needKeys(node *yaml.Node) []string {
	switch node.Kind {
	case yaml.ScalarNode:
		return []string{node.Value}
	case yaml.SequenceNode:
		keys := make([]string, 0, len(node.Content))
		for _, n := range node.Content {
			keys = append(keys, n.Value)
		}

		return keys
	default:
		return nil
	}
}

// nameMatches reports whether an API job name belongs to a display name,
// allowing a matrix suffix ("Tests (1/2)") or a reusable-workflow prefix
// ("Caller / Callee").
func nameMatches(api, display string) bool {
	return api == display || strings.HasPrefix(api, display+" (") || strings.HasPrefix(api, display+" / ") ||
		strings.HasSuffix(api, " / "+display)
}

func (g needsGraph) needsOf(r *jobRec) []string {
	for display, needs := range g[r.workflow] {
		if nameMatches(r.name, display) {
			return needs
		}
	}

	return nil
}

// ciChain is a sequence of jobs each waiting on the one before, ending at the
// last job to finish. Seconds is the sum of each link's median run time.
type ciChain struct {
	Chain  string  `json:"chain"`
	Pushes int     `json:"pushes"`
	Share  float64 `json:"share"`
}

// ciHandoff is the time between the last dependency of a job finishing and the
// job starting, split into GitHub scheduling and the runner queue.
type ciHandoff struct {
	Start      string `json:"start"`
	Chains     []ciChain
	Edges      int     `json:"edges"`
	P50Lag     float64 `json:"p50_lag_seconds"`
	P90Lag     float64 `json:"p90_lag_seconds"`
	P50Created float64 `json:"p50_created_after_seconds"`
	P50Queue   float64 `json:"p50_queue_seconds"`
}

// traceChain walks from a push's last job back through the dependency that
// finished last, and returns the jobs earliest first.
func traceChain(last *jobRec, byRun map[int64][]*jobRec, graph needsGraph) []string {
	chain := []string{last.name}
	cur := last

	for range maxChain {
		var next *jobRec

		for _, need := range graph.needsOf(cur) {
			for _, cand := range byRun[cur.runID] {
				if cand.ran() && nameMatches(cand.name, need) && (next == nil || cand.completed.After(next.completed)) {
					next = cand
				}
			}
		}

		if next == nil {
			break
		}

		chain = append(chain, next.name)
		cur = next
	}

	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	return chain
}

const maxChain = 10

func summarizeHandoffs(recs []jobRec, spans []timeWindow, graph needsGraph, ignore []string) []ciHandoff {
	out := make([]ciHandoff, 0, len(spans))

	for _, span := range spans {
		in := recsBetween(recs, span.from, span.to)
		byRun := make(map[int64][]*jobRec)

		for i := range in {
			byRun[in[i].runID] = append(byRun[in[i].runID], &in[i])
		}

		h := ciHandoff{Start: span.label}

		var lag, created, queue []float64

		for i := range in {
			r := &in[i]
			if !r.ran() {
				continue
			}

			dep := latestNeed(r, byRun, graph)
			if dep.IsZero() {
				continue
			}

			lag = append(lag, nonNegative(r.started.Sub(dep)))
			created = append(created, nonNegative(r.created.Sub(dep)))
			queue = append(queue, r.queue())
		}

		h.Edges = len(lag)
		h.P50Lag, h.P90Lag = percentile(lag, percentileMedian), percentile(lag, percentileP90)
		h.P50Created, h.P50Queue = percentile(created, percentileMedian), percentile(queue, percentileMedian)
		h.Chains = topPaths(in, graph, ignore)
		out = append(out, h)
	}

	return out
}

func latestNeed(r *jobRec, byRun map[int64][]*jobRec, graph needsGraph) time.Time {
	var latest time.Time

	for _, need := range graph.needsOf(r) {
		for _, cand := range byRun[r.runID] {
			if cand.ran() && nameMatches(cand.name, need) && cand.completed.After(latest) {
				latest = cand.completed
			}
		}
	}

	return latest
}

// topPaths counts, per push, the chain behind its last job to finish.
func topPaths(in []jobRec, graph needsGraph, ignore []string) []ciChain {
	byPush := make(map[string][]*jobRec)
	byRun := make(map[int64][]*jobRec)

	for i := range in {
		r := &in[i]
		byRun[r.runID] = append(byRun[r.runID], r)

		if r.ran() && !containsName(ignore, r.name) {
			byPush[r.push.HeadSHA] = append(byPush[r.push.HeadSHA], r)
		}
	}

	counts := make(map[string]int)

	for _, jobs := range byPush {
		last := jobs[0]
		for _, r := range jobs {
			if r.completed.After(last.completed) {
				last = r
			}
		}

		counts[last.workflow+": "+strings.Join(traceChain(last, byRun, graph), chainArrow)]++
	}

	out := make([]ciChain, 0, len(counts))
	for chain, n := range counts {
		out = append(out, ciChain{Chain: chain, Pushes: n, Share: float64(n) / float64(len(byPush))})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Pushes != out[j].Pushes {
			return out[i].Pushes > out[j].Pushes
		}

		return out[i].Chain < out[j].Chain
	})

	if len(out) > topChains {
		out = out[:topChains]
	}

	return out
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}

	return false
}

func writeHandoffs(b *strings.Builder, handoffs []ciHandoff) {
	if len(handoffs) == 0 {
		return
	}

	writeTableHead(b, "Handoff between dependent jobs (a `needs` edge: last dependency finishing to the job starting).",
		"<Epoch", "Edges", "Lag P50", "Lag P90", "Created after P50", "Queue P50")

	for i := range handoffs {
		h := &handoffs[i]
		fmt.Fprintf(b, "| %s | %d | %s | %s | %s | %s |\n", h.Start, h.Edges, formatSeconds(h.P50Lag),
			formatSeconds(h.P90Lag), formatSeconds(h.P50Created), formatSeconds(h.P50Queue))
	}

	writeTableHead(b, "Most common chains behind the last job to finish (earliest job first).",
		"<Epoch", "<Chain", "Pushes", "Share")

	for i := range handoffs {
		for _, c := range handoffs[i].Chains {
			fmt.Fprintf(b, "| %s | %s | %d | %.0f%% |\n", handoffs[i].Start, c.Chain, c.Pushes, c.Share*percentScale)
		}
	}
}
