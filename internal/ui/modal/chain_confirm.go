package modal

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// ChainConfirmResultMsg is sent when chain execution is confirmed or cancelled.
type ChainConfirmResultMsg struct {
	Confirmed bool
	ChainName string
	Variables map[string]string
	Branch    string
	Watch     bool
}

type chainConfirmKeyMap struct {
	Confirm key.Binding
	Cancel  key.Binding
	Watch   key.Binding
}

type resolvedStep struct {
	Workflow string
	Inputs   map[string]string
	Command  string
}

// ChainConfirmModal shows the chain configuration and confirms execution.
type ChainConfirmModal struct {
	chainName     string
	chain         *config.Chain
	variables     map[string]string
	branch        string
	watchMode     bool
	resolvedSteps []resolvedStep
	done          bool
	result        ChainConfirmResultMsg
	keys          chainConfirmKeyMap
}

// NewChainConfirmModal creates a chain confirmation modal.
func NewChainConfirmModal(chainName string, chainDef *config.Chain, variables map[string]string, branch string, watch bool) *ChainConfirmModal {
	m := &ChainConfirmModal{
		chainName: chainName,
		chain:     chainDef,
		variables: variables,
		branch:    branch,
		watchMode: watch,
		keys: chainConfirmKeyMap{
			Confirm: key.NewBinding(key.WithKeys("enter", "y")),
			Cancel:  key.NewBinding(key.WithKeys("esc", "n")),
			Watch:   key.NewBinding(key.WithKeys("w")),
		},
	}
	m.resolveSteps()
	return m
}

func (m *ChainConfirmModal) resolveSteps() {
	m.resolvedSteps = make([]resolvedStep, len(m.chain.Steps))

	ctx := &chain.InterpolationContext{
		Var:   m.variables,
		Steps: make(map[int]*chain.StepResult),
	}

	for i, step := range m.chain.Steps {
		inputs, _ := chain.InterpolateInputs(step.Inputs, ctx)

		cfg := runner.RunConfig{
			Workflow: step.Workflow,
			Branch:   m.branch,
			Inputs:   inputs,
		}
		args := runner.BuildArgs(cfg)

		m.resolvedSteps[i] = resolvedStep{
			Workflow: step.Workflow,
			Inputs:   inputs,
			Command:  runner.FormatCommand(args),
		}

		ctx.Steps[i] = &chain.StepResult{
			Workflow: step.Workflow,
			Inputs:   inputs,
		}
		if i > 0 {
			ctx.Previous = ctx.Steps[i-1]
		}
	}
}

// Update handles input for the chain confirm modal.
func (m *ChainConfirmModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Watch):
			m.watchMode = !m.watchMode
			return m, nil
		case key.Matches(msg, m.keys.Confirm):
			m.done = true
			m.result = ChainConfirmResultMsg{
				Confirmed: true,
				ChainName: m.chainName,
				Variables: m.variables,
				Branch:    m.branch,
				Watch:     m.watchMode,
			}
			return m, func() tea.Msg {
				return m.result
			}
		case key.Matches(msg, m.keys.Cancel):
			m.done = true
			m.result = ChainConfirmResultMsg{Confirmed: false}
			return m, func() tea.Msg {
				return m.result
			}
		}
	}
	return m, nil
}

// View renders the chain confirm modal.
func (m *ChainConfirmModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render("Confirm Chain Execution"))
	s.WriteString("\n\n")

	s.WriteString(ui.SubtitleStyle.Render("Chain: "))
	s.WriteString(ui.NormalStyle.Render(m.chainName))
	s.WriteString("\n")

	if m.chain.Description != "" {
		s.WriteString(ui.SubtitleStyle.Render("Description: "))
		s.WriteString(ui.TableDimmedStyle.Render(m.chain.Description))
		s.WriteString("\n")
	}

	s.WriteString(ui.SubtitleStyle.Render("Branch: "))
	branch := m.branch
	if branch == "" {
		branch = "(default)"
	}
	s.WriteString(ui.TableDimmedStyle.Render(branch))
	s.WriteString("\n\n")

	if len(m.variables) > 0 {
		s.WriteString(ui.SubtitleStyle.Render("Variables:"))
		s.WriteString("\n")

		keys := make([]string, 0, len(m.variables))
		for k := range m.variables {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := m.variables[k]
			s.WriteString(ui.NormalStyle.Render(fmt.Sprintf("  %s: ", k)))
			s.WriteString(ui.TableDimmedStyle.Render(v))
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	s.WriteString(ui.SubtitleStyle.Render("Steps:"))
	s.WriteString("\n")

	for i, step := range m.resolvedSteps {
		stepDef := m.chain.Steps[i]

		waitLabel := ""
		switch stepDef.WaitFor {
		case config.WaitSuccess:
			waitLabel = "(wait: success)"
		case config.WaitCompletion:
			waitLabel = "(wait: completion)"
		case config.WaitNone:
			waitLabel = "(wait: none)"
		}

		s.WriteString(ui.NormalStyle.Render(fmt.Sprintf("  %d. %s ", i+1, step.Workflow)))
		s.WriteString(ui.TableDimmedStyle.Render(waitLabel))
		s.WriteString("\n")
		s.WriteString(ui.CLIPreviewStyle.Render("     " + step.Command))
		s.WriteString("\n")
	}
	s.WriteString("\n")

	watchIndicator := "[ ]"
	if m.watchMode {
		watchIndicator = "[x]"
	}
	s.WriteString(ui.NormalStyle.Render(fmt.Sprintf("Watch runs: %s", watchIndicator)))
	s.WriteString("\n\n")

	s.WriteString(ui.HelpStyle.Render("[enter/y] confirm  [esc/n] cancel  [w] toggle watch"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *ChainConfirmModal) IsDone() bool {
	return m.done
}

// Result returns the confirmation result.
func (m *ChainConfirmModal) Result() any {
	return m.result
}
