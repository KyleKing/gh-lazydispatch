package modal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// ChainVariableResultMsg is sent when chain variable input is complete.
type ChainVariableResultMsg struct {
	Variables map[string]string
	ChainName string
	Cancelled bool
}

type chainVariableKeyMap struct {
	Confirm        key.Binding
	Cancel         key.Binding
	Up             key.Binding
	Down           key.Binding
	Edit           key.Binding
	RestoreDefault key.Binding
	Toggle         key.Binding
	NextOption     key.Binding
	PrevOption     key.Binding
}

// ChainVariableModal collects variable values for chain execution.
type ChainVariableModal struct {
	chainName     string
	chain         *config.Chain
	variables     map[string]string
	variableOrder []string
	selectedIndex int
	editing       bool
	editInput     textinput.Model
	done          bool
	result        ChainVariableResultMsg
	keys          chainVariableKeyMap
}

// NewChainVariableModal creates a chain variable input modal.
func NewChainVariableModal(chainName string, chainDef *config.Chain) *ChainVariableModal {
	variables := make(map[string]string)
	variableOrder := make([]string, len(chainDef.Variables))

	for i, v := range chainDef.Variables {
		variableOrder[i] = v.Name
		variables[v.Name] = v.Default
	}

	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 40
	ti.PromptStyle = ti.PromptStyle.UnsetBackground()
	ti.TextStyle = ti.TextStyle.UnsetBackground()
	ti.PlaceholderStyle = ti.PlaceholderStyle.UnsetBackground()
	ti.CompletionStyle = ti.CompletionStyle.UnsetBackground()
	ti.Cursor.Style = ti.Cursor.Style.UnsetBackground()

	return &ChainVariableModal{
		chainName:     chainName,
		chain:         chainDef,
		variables:     variables,
		variableOrder: variableOrder,
		selectedIndex: 0,
		editInput:     ti,
		keys: chainVariableKeyMap{
			Confirm:        key.NewBinding(key.WithKeys("enter")),
			Cancel:         key.NewBinding(key.WithKeys("esc")),
			Up:             key.NewBinding(key.WithKeys("up", "k")),
			Down:           key.NewBinding(key.WithKeys("down", "j")),
			Edit:           key.NewBinding(key.WithKeys("e", "enter")),
			RestoreDefault: key.NewBinding(key.WithKeys("ctrl+r")),
			Toggle:         key.NewBinding(key.WithKeys("space")),
			NextOption:     key.NewBinding(key.WithKeys("right", "l", "tab")),
			PrevOption:     key.NewBinding(key.WithKeys("left", "h", "shift+tab")),
		},
	}
}

func (m *ChainVariableModal) currentVariable() *config.ChainVariable {
	if m.selectedIndex >= len(m.chain.Variables) {
		return nil
	}
	return &m.chain.Variables[m.selectedIndex]
}

func (m *ChainVariableModal) currentName() string {
	if m.selectedIndex >= len(m.variableOrder) {
		return ""
	}
	return m.variableOrder[m.selectedIndex]
}

func (m *ChainVariableModal) validateRequired() []string {
	var missing []string
	for _, v := range m.chain.Variables {
		if v.Required && m.variables[v.Name] == "" {
			missing = append(missing, v.Name)
		}
	}
	return missing
}

// Update handles input for the chain variable modal.
func (m *ChainVariableModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	if m.editing {
		return m.updateEditing(msg)
	}
	return m.updateNavigating(msg)
}

func (m *ChainVariableModal) updateNavigating(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		v := m.currentVariable()
		name := m.currentName()

		switch {
		case key.Matches(msg, m.keys.Cancel):
			m.done = true
			m.result = ChainVariableResultMsg{Cancelled: true}
			return m, func() tea.Msg { return m.result }

		case key.Matches(msg, m.keys.Up):
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if m.selectedIndex < len(m.variableOrder)-1 {
				m.selectedIndex++
			}
			return m, nil

		case key.Matches(msg, m.keys.RestoreDefault):
			if v != nil {
				m.variables[name] = v.Default
			}
			return m, nil

		case key.Matches(msg, m.keys.Toggle):
			if v != nil && v.Type == "boolean" {
				if m.variables[name] == "true" {
					m.variables[name] = "false"
				} else {
					m.variables[name] = "true"
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.NextOption):
			if v != nil && v.Type == "choice" && len(v.Options) > 0 {
				m.cycleOption(name, v.Options, 1)
			}
			return m, nil

		case key.Matches(msg, m.keys.PrevOption):
			if v != nil && v.Type == "choice" && len(v.Options) > 0 {
				m.cycleOption(name, v.Options, -1)
			}
			return m, nil

		case key.Matches(msg, m.keys.Edit), key.Matches(msg, m.keys.Confirm):
			if v == nil {
				return m, nil
			}

			switch v.Type {
			case "string":
				m.editing = true
				m.editInput.SetValue(m.variables[name])
				m.editInput.Focus()
				return m, nil

			case "boolean":
				if m.variables[name] == "true" {
					m.variables[name] = "false"
				} else {
					m.variables[name] = "true"
				}
				return m.advanceOrConfirm()

			case "choice":
				if len(v.Options) > 0 {
					m.cycleOption(name, v.Options, 1)
				}
				return m.advanceOrConfirm()

			default:
				m.editing = true
				m.editInput.SetValue(m.variables[name])
				m.editInput.Focus()
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *ChainVariableModal) cycleOption(name string, options []string, delta int) {
	currentIdx := 0
	for i, opt := range options {
		if opt == m.variables[name] {
			currentIdx = i
			break
		}
	}
	newIdx := (currentIdx + delta + len(options)) % len(options)
	m.variables[name] = options[newIdx]
}

func (m *ChainVariableModal) advanceOrConfirm() (Context, tea.Cmd) {
	if m.selectedIndex < len(m.variableOrder)-1 {
		m.selectedIndex++
		return m, nil
	}
	return m.tryConfirm()
}

func (m *ChainVariableModal) tryConfirm() (Context, tea.Cmd) {
	missing := m.validateRequired()
	if len(missing) > 0 {
		return m, nil
	}

	m.done = true
	m.result = ChainVariableResultMsg{
		Variables: m.variables,
		ChainName: m.chainName,
		Cancelled: false,
	}
	return m, func() tea.Msg { return m.result }
}

func (m *ChainVariableModal) updateEditing(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			name := m.currentName()
			m.variables[name] = m.editInput.Value()
			m.editing = false
			m.editInput.Blur()
			return m.advanceOrConfirm()

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.editing = false
			m.editInput.Blur()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

// View renders the chain variable modal.
func (m *ChainVariableModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render(fmt.Sprintf("Configure Chain: %s", m.chainName)))
	s.WriteString("\n")

	if m.chain.Description != "" {
		s.WriteString(ui.SubtitleStyle.Render(m.chain.Description))
		s.WriteString("\n")
	}
	s.WriteString("\n")

	s.WriteString(ui.SubtitleStyle.Render("Variables:"))
	s.WriteString("\n\n")

	for i, v := range m.chain.Variables {
		indicator := "  "
		if i == m.selectedIndex {
			indicator = "> "
		}

		name := v.Name
		if v.Required {
			name += "*"
		}

		value := m.variables[v.Name]
		if value == "" {
			value = `("")`
		}

		var rowStyle = ui.TableRowStyle
		if i == m.selectedIndex {
			rowStyle = ui.TableSelectedStyle
		}

		typeHint := ""
		switch v.Type {
		case "boolean":
			typeHint = " [space: toggle]"
		case "choice":
			typeHint = " [←→: cycle]"
		}

		row := fmt.Sprintf("%s%-15s = %s", indicator, name, value)
		s.WriteString(rowStyle.Render(row))
		if i == m.selectedIndex && !m.editing {
			s.WriteString(ui.TableDimmedStyle.Render(typeHint))
		}
		s.WriteString("\n")

		if v.Description != "" && i == m.selectedIndex {
			s.WriteString(ui.SubtitleStyle.Render(fmt.Sprintf("   %s", v.Description)))
			s.WriteString("\n")
		}

		if v.Type == "choice" && len(v.Options) > 0 && i == m.selectedIndex {
			s.WriteString(ui.TableDimmedStyle.Render(fmt.Sprintf("   Options: %s", strings.Join(v.Options, ", "))))
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")

	if m.editing {
		s.WriteString(ui.SubtitleStyle.Render("Editing:"))
		s.WriteString("\n")
		s.WriteString(m.editInput.View())
		s.WriteString("\n\n")
		s.WriteString(ui.HelpStyle.Render("[enter] save  [esc] cancel"))
	} else {
		missing := m.validateRequired()
		if len(missing) > 0 {
			s.WriteString(ui.SelectedStyle.Render(fmt.Sprintf("Required: %s", strings.Join(missing, ", "))))
			s.WriteString("\n\n")
		}
		s.WriteString(ui.HelpStyle.Render("[↑↓] navigate  [enter/e] edit  [ctrl+r] default  [esc] cancel"))
	}

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *ChainVariableModal) IsDone() bool {
	return m.done
}

// Result returns the variable collection result.
func (m *ChainVariableModal) Result() any {
	return m.result
}
