package modal

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// ChainVariableResultMsg is sent when chain variable input is complete.
type ChainVariableResultMsg struct {
	Variables map[string]string
	ChainName string
	Canceled  bool
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
	chain         *config.Chain
	variables     map[string]string
	chainName     string
	keys          chainVariableKeyMap
	result        ChainVariableResultMsg
	variableOrder []string
	editInput     textinput.Model
	selectedIndex int
	editing       bool
	done          bool
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
	ti.SetWidth(defaultTextInputWidth)
	s := ti.Styles()
	s.Focused.Prompt = s.Focused.Prompt.UnsetBackground()
	s.Focused.Text = s.Focused.Text.UnsetBackground()
	s.Focused.Placeholder = s.Focused.Placeholder.UnsetBackground()
	s.Focused.Suggestion = s.Focused.Suggestion.UnsetBackground()
	s.Blurred.Prompt = s.Blurred.Prompt.UnsetBackground()
	s.Blurred.Text = s.Blurred.Text.UnsetBackground()
	s.Blurred.Placeholder = s.Blurred.Placeholder.UnsetBackground()
	s.Blurred.Suggestion = s.Blurred.Suggestion.UnsetBackground()
	ti.SetStyles(s)

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
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, m.keys.Cancel):
		m.done = true
		m.result = ChainVariableResultMsg{Canceled: true}

		return m, func() tea.Msg { return m.result }

	case key.Matches(keyMsg, m.keys.Up):
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}

		return m, nil

	case key.Matches(keyMsg, m.keys.Down):
		if m.selectedIndex < len(m.variableOrder)-1 {
			m.selectedIndex++
		}

		return m, nil
	}

	return m.updateVariableValue(keyMsg)
}

// updateVariableValue handles keys that modify the currently selected variable's value.
func (m *ChainVariableModal) updateVariableValue(keyMsg tea.KeyPressMsg) (Context, tea.Cmd) {
	v := m.currentVariable()
	name := m.currentName()

	switch {
	case key.Matches(keyMsg, m.keys.RestoreDefault):
		if v != nil {
			m.variables[name] = v.Default
		}

		return m, nil

	case key.Matches(keyMsg, m.keys.Toggle):
		if v != nil && v.Type == inputTypeBoolean {
			m.toggleBoolVariable(name)
		}

		return m, nil

	case key.Matches(keyMsg, m.keys.NextOption):
		if v != nil && v.Type == inputTypeChoice && len(v.Options) > 0 {
			m.cycleOption(name, v.Options, 1)
		}

		return m, nil

	case key.Matches(keyMsg, m.keys.PrevOption):
		if v != nil && v.Type == inputTypeChoice && len(v.Options) > 0 {
			m.cycleOption(name, v.Options, -1)
		}

		return m, nil

	case key.Matches(keyMsg, m.keys.Edit), key.Matches(keyMsg, m.keys.Confirm):
		return m.handleEditOrConfirm(v, name)
	}

	return m, nil
}

// toggleBoolVariable flips a boolean-typed variable between "true" and "false".
func (m *ChainVariableModal) toggleBoolVariable(name string) {
	if m.variables[name] == boolTrueValue {
		m.variables[name] = "false"
	} else {
		m.variables[name] = boolTrueValue
	}
}

// startEditing switches the modal into free-text editing mode for the given variable.
func (m *ChainVariableModal) startEditing(name string) (Context, tea.Cmd) {
	m.editing = true
	m.editInput.SetValue(m.variables[name])
	m.editInput.Focus()

	return m, nil
}

// handleEditOrConfirm dispatches the Edit/Confirm key based on the current variable's type.
func (m *ChainVariableModal) handleEditOrConfirm(v *config.ChainVariable, name string) (Context, tea.Cmd) {
	if v == nil {
		return m, nil
	}

	switch v.Type {
	case "string":
		return m.startEditing(name)

	case inputTypeBoolean:
		m.toggleBoolVariable(name)

		return m.advanceOrConfirm()

	case inputTypeChoice:
		if len(v.Options) > 0 {
			m.cycleOption(name, v.Options, 1)
		}

		return m.advanceOrConfirm()

	default:
		return m.startEditing(name)
	}
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
		Canceled:  false,
	}

	return m, func() tea.Msg { return m.result }
}

func (m *ChainVariableModal) updateEditing(msg tea.Msg) (Context, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
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

	s.WriteString(ui.TitleStyle.Render("Configure Chain: " + m.chainName))
	s.WriteString("\n")

	if m.chain.Description != "" {
		s.WriteString(ui.SubtitleStyle.Render(m.chain.Description))
		s.WriteString("\n")
	}

	s.WriteString("\n")

	s.WriteString(ui.SubtitleStyle.Render("Variables:"))
	s.WriteString("\n\n")

	for i, v := range m.chain.Variables {
		m.renderVariableRow(&s, i, v)
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
			s.WriteString(ui.SelectedStyle.Render("Required: " + strings.Join(missing, ", ")))
			s.WriteString("\n\n")
		}

		s.WriteString(ui.HelpStyle.Render("[↑↓] navigate  [enter/e] edit  [ctrl+r] default  [esc] cancel"))
	}

	return s.String()
}

// renderVariableRow writes a single chain variable's row (name/value, type
// hint, description, and options) to s.
func (m *ChainVariableModal) renderVariableRow(s *strings.Builder, i int, v config.ChainVariable) {
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

	rowStyle := ui.TableRowStyle
	if i == m.selectedIndex {
		rowStyle = ui.TableSelectedStyle
	}

	typeHint := ""

	switch v.Type {
	case inputTypeBoolean:
		typeHint = " [space: toggle]"
	case inputTypeChoice:
		typeHint = " [←→: cycle]"
	}

	row := fmt.Sprintf("%s%-15s = %s", indicator, name, value)
	s.WriteString(rowStyle.Render(row))

	if i == m.selectedIndex && !m.editing {
		s.WriteString(ui.TableDimmedStyle.Render(typeHint))
	}

	s.WriteString("\n")

	if v.Description != "" && i == m.selectedIndex {
		s.WriteString(ui.SubtitleStyle.Render("   " + v.Description))
		s.WriteString("\n")
	}

	if v.Type == inputTypeChoice && len(v.Options) > 0 && i == m.selectedIndex {
		s.WriteString(ui.TableDimmedStyle.Render("   Options: " + strings.Join(v.Options, ", ")))
		s.WriteString("\n")
	}
}

// IsDone returns true if the modal is finished.
func (m *ChainVariableModal) IsDone() bool {
	return m.done
}

// Result returns the variable collection result.
func (m *ChainVariableModal) Result() any {
	return m.result
}
