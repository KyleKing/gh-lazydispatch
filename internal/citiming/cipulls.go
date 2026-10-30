package citiming

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// prCandidatePool is how many of an author's most-pushed PRs have their size
// read before the largest are kept.
const prCandidatePool = 8

const titleWidth = 48

// ciPR is one pull request's size beside the CI time of its pushes.
type ciPR struct {
	FirstPush string `json:"first_push"`
	Slowest   string `json:"slowest_workflow,omitempty"`
	github.PRStats
	Pushes          int     `json:"pushes"`
	P50Seconds      float64 `json:"p50_seconds"`
	P90Seconds      float64 `json:"p90_seconds"`
	SlowestP90Final float64 `json:"slowest_workflow_p90_seconds,omitempty"`
}

type prReader interface {
	GetPRStats(number int) (github.PRStats, error)
}

// pushesByPR groups counted commits by the pull request they belong to.
func pushesByPR(commits []ciTiming) map[int][]ciTiming {
	byPR := make(map[int][]ciTiming)

	kept := counted(commits)
	for i := range kept {
		if len(kept[i].PRs) > 0 {
			byPR[kept[i].PRs[0]] = append(byPR[kept[i].PRs[0]], kept[i])
		}
	}

	return byPR
}

// samplePRs reads the size of the explicit PRs, then of each author's
// perAuthor largest PRs among their most-pushed ones.
func samplePRs(reader prReader, commits []ciTiming, logins []string, explicit []int, perAuthor int) ([]ciPR, error) {
	byPR := pushesByPR(commits)
	numbers := append([]int(nil), explicit...)
	known := make(map[int]github.PRStats)

	for _, login := range logins {
		pool := prsOf(byPR, login)
		if len(pool) > prCandidatePool {
			pool = pool[:prCandidatePool]
		}

		sized := make([]github.PRStats, 0, len(pool))

		for _, n := range pool {
			stats, err := reader.GetPRStats(n)
			if err != nil {
				return nil, fmt.Errorf("sizing candidate pull request: %w", err)
			}

			known[n] = stats

			if stats.Author == login {
				sized = append(sized, stats)
			}
		}

		sort.SliceStable(sized, func(i, j int) bool { return sized[i].ChangedFiles > sized[j].ChangedFiles })

		for i := 0; i < len(sized) && i < perAuthor; i++ {
			numbers = append(numbers, sized[i].Number)
		}
	}

	out := make([]ciPR, 0, len(numbers))

	for _, n := range numbers {
		stats, ok := known[n]
		if !ok {
			var err error
			if stats, err = reader.GetPRStats(n); err != nil {
				return nil, fmt.Errorf("sizing pull request: %w", err)
			}
		}

		out = append(out, summarizePR(stats, byPR[n]))
	}

	return out, nil
}

// prsOf lists login's PR numbers, most pushes first.
func prsOf(byPR map[int][]ciTiming, login string) []int {
	var numbers []int

	for n, pushes := range byPR {
		if pushes[0].Actor == login {
			numbers = append(numbers, n)
		}
	}

	sort.Slice(numbers, func(i, j int) bool {
		a, b := len(byPR[numbers[i]]), len(byPR[numbers[j]])
		if a != b {
			return a > b
		}

		return numbers[i] < numbers[j]
	})

	return numbers
}

func summarizePR(stats github.PRStats, pushes []ciTiming) ciPR {
	pr := ciPR{
		PRStats:    stats,
		Pushes:     len(pushes),
		P50Seconds: secondsPercentile(pushes, percentileMedian),
		P90Seconds: secondsPercentile(pushes, percentileP90),
	}

	if len(pushes) == 0 {
		return pr
	}

	pr.FirstPush = pushes[0].StartedAt

	if workflows := summarizeWorkflows(pushes); len(workflows) > 0 {
		pr.Slowest, pr.SlowestP90Final = workflows[0].Name, workflows[0].P90FinishedAfter
	}

	return pr
}

func writePRTable(b *strings.Builder, prs []ciPR) {
	if len(prs) == 0 {
		return
	}

	b.WriteString("\nPull requests spot-checked (CI time per push).\n\n")
	b.WriteString("| PR | Author | Title | Files | Lines | Commits | Pushes timed | First push |")
	b.WriteString(" P50 | P90 | Slowest workflow |\n")
	b.WriteString("| --- | --- | --- | ---: | ---: | ---: | ---: | --- | ---: | ---: | --- |\n")

	for i := range prs {
		pr := &prs[i]

		first, slowest := "-", "-"
		if pr.FirstPush != "" {
			first = pr.FirstPush[:dateLen]
			slowest = fmt.Sprintf("%s (P90 %s)", pr.Slowest, formatSeconds(pr.SlowestP90Final))
		}

		fmt.Fprintf(b, "| #%d | %s | %s | %d | +%d/-%d | %d | %d | %s | %s | %s | %s |\n", pr.Number, pr.Author,
			truncate(pr.Title, titleWidth), pr.ChangedFiles, pr.Additions, pr.Deletions, pr.Commits, pr.Pushes, first,
			formatSeconds(pr.P50Seconds), formatSeconds(pr.P90Seconds), slowest)
	}
}

func truncate(s string, width int) string {
	s = strings.ReplaceAll(s, "|", "/")
	if r := []rune(s); len(r) > width {
		return string(r[:width-1]) + "~"
	}

	return s
}
