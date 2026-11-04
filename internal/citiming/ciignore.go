package citiming

import (
	"sort"
	"strconv"
	"strings"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// ignoredRuns is what dropIgnored removed: runs per workflow name, and the
// workflow files those runs came from.
type ignoredRuns struct {
	counts map[string]int
	paths  []string
}

// dropIgnored removes the runs of workflows the caller says never gate a push
// (bot, label, and nudge workflows), matched by display name, file name, or
// file stem. Their time then counts toward no push, and their files cut no epoch.
func dropIgnored(runs []github.RunTiming, ignore []string) ([]github.RunTiming, ignoredRuns) {
	if len(ignore) == 0 {
		return runs, ignoredRuns{}
	}

	dropped := ignoredRuns{counts: make(map[string]int)}
	seen := make(map[string]bool)
	kept := make([]github.RunTiming, 0, len(runs))

	for i := range runs {
		if !matchesWorkflow(&runs[i], ignore) {
			kept = append(kept, runs[i])

			continue
		}

		dropped.counts[runs[i].Name]++

		if file := runFile(runs[i].Path); file != "" && !seen[file] {
			seen[file] = true
			dropped.paths = append(dropped.paths, file)
		}
	}

	sort.Strings(dropped.paths)

	return kept, dropped
}

func matchesWorkflow(run *github.RunTiming, names []string) bool {
	file := runFile(run.Path)
	base := file[strings.LastIndex(file, "/")+1:]

	for _, name := range names {
		if strings.EqualFold(run.Name, name) || (file != "" && (name == base || name == file ||
			name == workflowFileStem(file))) {
			return true
		}
	}

	return false
}

// runFile drops the "@ref" a run's path carries for a reusable workflow.
func runFile(path string) string {
	file, _, _ := strings.Cut(path, "@")

	return file
}

func writeIgnored(b *strings.Builder, ignored map[string]int) {
	if len(ignored) == 0 {
		return
	}

	names := make([]string, 0, len(ignored))
	for name, n := range ignored {
		names = append(names, name+" ("+strconv.Itoa(n)+" runs)")
	}

	sort.Strings(names)

	b.WriteString("\nLeft out of the wall clock and the epoch cuts as workflows that never gate a push: " +
		strings.Join(names, ", ") + ".\n")
}
