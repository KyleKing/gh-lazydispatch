package citiming

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

const dateLen = len("2006-01-02")

// ciAuthor is one person's CI timing, cut by window, epoch, and day.
type ciAuthor struct {
	Login   string           `json:"login"`
	Windows []ciTimingWindow `json:"windows"`
	Epochs  []ciEpoch        `json:"epochs,omitempty"`
	Daily   []ciDay          `json:"daily,omitempty"`
}

type ciDay struct {
	Date       string  `json:"date"`
	Commits    int     `json:"commits"`
	P50Seconds float64 `json:"p50_seconds"`
	P90Seconds float64 `json:"p90_seconds"`
}

// ciWorkflowEpochs is one workflow file's run durations, cut at the commits
// that changed that file alone.
type ciWorkflowEpochs struct {
	Workflow string    `json:"workflow"`
	Path     string    `json:"path"`
	Epochs   []ciEpoch `json:"epochs"`
}

func commitsBy(commits []ciTiming, login string) []ciTiming {
	var out []ciTiming

	for i := range commits {
		if commits[i].Actor == login {
			out = append(out, commits[i])
		}
	}

	return out
}

// summarizeAuthors cuts each author's commits at the windows and at the
// already-folded epoch dates, so every author's rows line up.
func summarizeAuthors(
	commits []ciTiming, logins []string, windows [2]timeWindow, epochs []ciEpoch, since time.Time,
) []ciAuthor {
	bounds := boundsOf(epochs)
	authors := make([]ciAuthor, 0, len(logins))

	for _, login := range logins {
		theirs := commitsBy(commits, login)

		author := ciAuthor{
			Login:   login,
			Windows: summarizeWindows(theirs, windows),
			Daily:   summarizeDays(theirs, since),
		}

		if len(bounds) > 0 {
			author.Epochs = summarizeEpochs(theirs, bounds, 0)
			for i := range author.Epochs {
				author.Epochs[i].Few = author.Epochs[i].Commits < MinEpochCommits
			}
		}

		authors = append(authors, author)
	}

	return authors
}

func summarizeDays(commits []ciTiming, since time.Time) []ciDay {
	byDay := make(map[string][]ciTiming)

	kept := counted(commits)
	for i := range kept {
		if started, err := time.Parse(timeFormat, kept[i].StartedAt); err == nil && !started.Before(since) {
			byDay[kept[i].StartedAt[:dateLen]] = append(byDay[kept[i].StartedAt[:dateLen]], kept[i])
		}
	}

	days := make([]ciDay, 0, len(byDay))

	for date, in := range byDay {
		days = append(days, ciDay{
			Date:       date,
			Commits:    len(in),
			P50Seconds: secondsPercentile(in, percentileMedian),
			P90Seconds: secondsPercentile(in, percentileP90),
		})
	}

	sort.Slice(days, func(i, j int) bool { return days[i].Date < days[j].Date })

	return days
}

// workflowEpochs cuts each workflow's run durations at the commits that touched
// its own file. A workflow whose file did not change in range has no epochs.
func workflowEpochs(
	ctx context.Context, branch string, since time.Time, runs []github.RunTiming,
) ([]ciWorkflowEpochs, error) {
	byPath := make(map[string][]github.RunTiming)
	names := make(map[string]string)

	for i := range runs {
		if runs[i].Path != "" {
			byPath[runs[i].Path] = append(byPath[runs[i].Path], runs[i])
			names[runs[i].Path] = runs[i].Name
		}
	}

	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}

	sort.Strings(paths)

	var out []ciWorkflowEpochs

	for _, path := range paths {
		bounds, err := pathBoundaries(ctx, branch, since, path)
		if err != nil {
			return nil, err
		}

		if len(bounds) <= 1 {
			continue
		}

		samples := make([]ciTiming, 0, len(byPath[path]))
		for i := range byPath[path] {
			run := byPath[path][i]
			samples = append(samples, ciTiming{
				StartedAt: run.CreatedAt.Format(timeFormat),
				Seconds:   run.UpdatedAt.Sub(run.CreatedAt).Seconds(),
				Excluded:  exclusion([]github.RunTiming{run}),
			})
		}

		out = append(out, ciWorkflowEpochs{
			Workflow: names[path],
			Path:     path,
			Epochs:   summarizeEpochs(samples, bounds, MinEpochCommits),
		})
	}

	return out, nil
}

func writeWorkflowEpochTable(b *strings.Builder, workflows []ciWorkflowEpochs) {
	if len(workflows) == 0 {
		return
	}

	b.WriteString("\nWorkflow epochs (run duration, cut only at commits that changed that workflow's file).\n\n")
	b.WriteString("| Workflow | Start | Change | Runs | P50 | P90 |\n| --- | --- | --- | ---: | ---: | ---: |\n")

	for i := range workflows {
		for j := range workflows[i].Epochs {
			e := &workflows[i].Epochs[j]
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s |\n", workflows[i].Workflow, e.Start[:dateLen],
				epochLabel(e), epochCount(e), formatSeconds(e.P50Seconds), formatSeconds(e.P90Seconds))
		}
	}
}

func writeAuthorTables(b *strings.Builder, authors []ciAuthor) {
	if len(authors) == 0 {
		return
	}

	b.WriteString("\nBy author, baseline against current window.\n\n")
	b.WriteString("| Author | Baseline commits | P50 | P90 | Current commits | P50 | P90 | P90 change |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")

	for i := range authors {
		w := authors[i].Windows

		change := "-"
		if w[0].P90Seconds > 0 && w[1].Commits > 0 {
			change = fmt.Sprintf("%+.0f%%", (w[1].P90Seconds/w[0].P90Seconds-1)*percentScale)
		}

		fmt.Fprintf(b, "| %s | %d | %s | %s | %d | %s | %s | %s |\n", authors[i].Login,
			w[0].Commits, formatSeconds(w[0].P50Seconds), formatSeconds(w[0].P90Seconds),
			w[1].Commits, formatSeconds(w[1].P50Seconds), formatSeconds(w[1].P90Seconds), change)
	}

	writeAuthorEpochs(b, authors)
	writeAuthorDays(b, authors)
}

func authorColumns(authors []ciAuthor) (string, string) {
	var header, rule strings.Builder

	for i := range authors {
		fmt.Fprintf(&header, " %s n | P50 | P90 |", authors[i].Login)
		rule.WriteString(" ---: | ---: | ---: |")
	}

	return header.String(), rule.String()
}

func writeAuthorEpochs(b *strings.Builder, authors []ciAuthor) {
	if len(authors[0].Epochs) == 0 {
		return
	}

	header, rule := authorColumns(authors)

	fmt.Fprintf(b, "\nBy author and epoch (n under %d is noise).\n\n| Start | Change |%s\n| --- | --- |%s\n",
		MinEpochCommits, header, rule)

	for row := range authors[0].Epochs {
		first := &authors[0].Epochs[row]
		fmt.Fprintf(b, "| %s | %s |", first.Start[:dateLen], epochLabel(first))

		for i := range authors {
			if row >= len(authors[i].Epochs) {
				b.WriteString(" - | - | - |")

				continue
			}

			e := &authors[i].Epochs[row]
			fmt.Fprintf(b, " %s | %s | %s |", epochCount(e), formatSeconds(e.P50Seconds), formatSeconds(e.P90Seconds))
		}

		b.WriteString("\n")
	}
}

func writeAuthorDays(b *strings.Builder, authors []ciAuthor) {
	dates := make(map[string]bool)

	for i := range authors {
		for _, d := range authors[i].Daily {
			dates[d.Date] = true
		}
	}

	if len(dates) == 0 {
		return
	}

	ordered := make([]string, 0, len(dates))
	for date := range dates {
		ordered = append(ordered, date)
	}

	sort.Strings(ordered)

	header, rule := authorColumns(authors)

	fmt.Fprintf(b, "\nBy author and day (when it changed).\n\n| Day |%s\n| --- |%s\n", header, rule)

	for _, date := range ordered {
		fmt.Fprintf(b, "| %s |", date)

		for i := range authors {
			cell := " - | - | - |"

			for _, d := range authors[i].Daily {
				if d.Date == date {
					cell = fmt.Sprintf(" %d | %s | %s |", d.Commits,
						formatSeconds(d.P50Seconds), formatSeconds(d.P90Seconds))
				}
			}

			b.WriteString(cell)
		}

		b.WriteString("\n")
	}
}
