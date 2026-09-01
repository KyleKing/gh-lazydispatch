package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/git"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
)

// DefaultRegistry builds the commands the `:` bar offers. Every one of them
// is reachable by a key as well; the bar exists so an action has a name to
// guess at when its key is not the one you remember.
func DefaultRegistry() Registry {
	return NewRegistry(
		branchCommand(),
		chainCommand(),
		Command{
			Name:        "filter",
			Description: "Filter the config pane's inputs",
			Run: func(m Model, args []string) (Model, tea.Cmd) {
				m.filterText = strings.Join(args, " ")
				m.applyFilter()

				return m, nil
			},
		},
		Command{
			Name:        "help",
			Description: "Show the keyboard reference",
			Run: func(m Model, _ []string) (Model, tea.Cmd) {
				m.modalStack.Push(modal.NewHelpModal())

				return m, nil
			},
		},
		Command{
			Name:        "quit",
			Description: "Leave gh-lazydispatch",
			Run:         func(m Model, _ []string) (Model, tea.Cmd) { return m, tea.Quit },
		},
		Command{
			Name:        "reset",
			Description: "Restore every input to its workflow default",
			Run: func(m Model, _ []string) (Model, tea.Cmd) {
				m.resetAllInputs()

				return m, nil
			},
		},
		Command{
			Name:        "watch",
			Description: "Toggle watching the run after dispatch",
			Run: func(m Model, _ []string) (Model, tea.Cmd) {
				m.watchRun = !m.watchRun

				return m, nil
			},
		},
		workflowCommand(),
	)
}

func branchCommand() Command {
	return Command{
		Name:        nameBranch,
		Description: "Dispatch against a different branch: :branch <name>",
		Complete: func(_ Model, args []string) []Candidate {
			branches, err := git.FetchBranches(context.Background())
			if err != nil {
				branches = git.DefaultBranches()
			}

			return matchingCandidates(branches, lastArg(args), nameBranch)
		},
		Run: func(m Model, args []string) (Model, tea.Cmd) {
			if len(args) == 0 {
				return m, statusCmd("usage: :branch <name>")
			}

			m.branch = args[0]

			return m, nil
		},
	}
}

func chainCommand() Command {
	return Command{
		Name:        "chain",
		Description: "Run a chain from .github/lazydispatch.yml: :chain <name>",
		Complete: func(m Model, args []string) []Candidate {
			if m.wfdConfig == nil {
				return nil
			}

			candidates := make([]Candidate, 0, len(m.wfdConfig.Chains))
			for _, name := range m.wfdConfig.ChainNames() {
				candidates = append(candidates,
					Candidate{Name: name, Description: m.wfdConfig.Chains[name].Description})
			}

			return filterCandidates(candidates, lastArg(args))
		},
		Run: func(m Model, args []string) (Model, tea.Cmd) {
			if len(args) == 0 || m.wfdConfig == nil {
				return m, statusCmd("usage: :chain <name>")
			}

			chainDef, ok := m.wfdConfig.Chains[args[0]]
			if !ok {
				return m, statusCmd(fmt.Sprintf("no chain named %q", args[0]))
			}

			model, cmd := m.startChainFlow(args[0], chainDef)

			return asModelOrSelf(model, m), cmd
		},
	}
}

func workflowCommand() Command {
	return Command{
		Name:        nameWorkflow,
		Description: "Select a workflow by filename: :workflow <file>",
		Complete: func(m Model, args []string) []Candidate {
			candidates := make([]Candidate, 0, len(m.workflows))
			for _, wf := range m.workflows {
				candidates = append(candidates, Candidate{Name: wf.Filename, Description: wf.Name})
			}

			return filterCandidates(candidates, lastArg(args))
		},
		Run: func(m Model, args []string) (Model, tea.Cmd) {
			if len(args) == 0 {
				return m, statusCmd("usage: :workflow <file>")
			}

			for i, wf := range m.workflows {
				if wf.Filename != args[0] {
					continue
				}

				m.selectedWorkflow = i
				m.initializeInputs(wf)
				m.syncHistoryEntries()

				return m, nil
			}

			return m, statusCmd(fmt.Sprintf("no workflow named %q", args[0]))
		},
	}
}

// lastArg is the word being completed, which is the last one the bar handed
// over and may be empty when a new word has just been opened.
func lastArg(args []string) string {
	if len(args) == 0 {
		return ""
	}

	return args[len(args)-1]
}

func matchingCandidates(names []string, prefix, description string) []Candidate {
	candidates := make([]Candidate, 0, len(names))
	for _, name := range names {
		candidates = append(candidates, Candidate{Name: name, Description: description})
	}

	return filterCandidates(candidates, prefix)
}

// filterCandidates keeps the candidates starting with prefix, in name order.
func filterCandidates(candidates []Candidate, prefix string) []Candidate {
	kept := make([]Candidate, 0, len(candidates))

	for _, candidate := range candidates {
		if strings.HasPrefix(candidate.Name, prefix) {
			kept = append(kept, candidate)
		}
	}

	sort.Slice(kept, func(i, j int) bool { return kept[i].Name < kept[j].Name })

	return kept
}

// asModelOrSelf unwraps a handler's tea.Model back to this package's Model,
// falling back to the caller's own when a handler returns something else.
func asModelOrSelf(result tea.Model, fallback Model) Model {
	if m, ok := result.(Model); ok {
		return m
	}

	return fallback
}
