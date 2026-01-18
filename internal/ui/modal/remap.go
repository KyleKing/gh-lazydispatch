package modal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/lazydispatch/internal/ui"
	"github.com/kyleking/lazydispatch/internal/validation"
	"github.com/kyleking/lazydispatch/internal/workflow"
)

// RemapAction represents the action to take for a validation error.
type RemapAction int

const (
	RemapActionDrop RemapAction = iota // Drop this input
	RemapActionKeep                    // Keep with original name (ignore error)
	RemapActionMap                     // Map to a different input name
)

// RemapDecision represents a user decision for a validation error.
type RemapDecision struct {
	OriginalName string
	Action       RemapAction
	NewName      string // Used when Action is RemapActionMap
}

// RemapModal presents a wizard for remapping invalid configuration inputs.
type RemapModal struct {
	errors          []validation.ConfigValidationError
	currentInputs   map[string]workflow.WorkflowInput
	currentErrorIdx int
	decisions       []RemapDecision
	selected        int
	options         []remapOption
	done            bool
	cancelled       bool
	keys            remapKeyMap
}

type remapOption struct {
	label       string
	action      RemapAction
	targetName  string
	description string
}

type remapKeyMap struct {
	Down   key.Binding
	Enter  key.Binding
	Escape key.Binding
	Up     key.Binding
}

func defaultRemapKeyMap() remapKeyMap {
	return remapKeyMap{
		Down:   key.NewBinding(key.WithKeys("down", "j")),
		Enter:  key.NewBinding(key.WithKeys("enter")),
		Escape: key.NewBinding(key.WithKeys("esc")),
		Up:     key.NewBinding(key.WithKeys("up", "k")),
	}
}

// NewRemapModal creates a new remapping wizard modal.
func NewRemapModal(errors []validation.ConfigValidationError, currentInputs map[string]workflow.WorkflowInput) *RemapModal {
	m := &RemapModal{
		errors:          errors,
		currentInputs:   currentInputs,
		currentErrorIdx: 0,
		decisions:       make([]RemapDecision, 0, len(errors)),
		keys:            defaultRemapKeyMap(),
	}
	m.buildOptions()
	return m
}

// buildOptions creates the list of options for the current error.
func (m *RemapModal) buildOptions() {
	if m.currentErrorIdx >= len(m.errors) {
		m.options = nil
		return
	}

	err := m.errors[m.currentErrorIdx]
	m.options = make([]remapOption, 0)

	// Option 1: Drop this input
	m.options = append(m.options, remapOption{
		label:       "Drop this input",
		action:      RemapActionDrop,
		description: "Remove from configuration",
	})

	// Option 2: Keep original (ignore error)
	m.options = append(m.options, remapOption{
		label:       "Keep original (ignore error)",
		action:      RemapActionKeep,
		description: "Apply configuration as-is",
	})

	// Option 3: Map to suggestion (if available)
	if err.Suggestion != "" {
		m.options = append(m.options, remapOption{
			label:       fmt.Sprintf("Map to: %s", err.Suggestion),
			action:      RemapActionMap,
			targetName:  err.Suggestion,
			description: "Use suggested input name",
		})
	}

	// Options 4+: Map to any other valid input
	for name := range m.currentInputs {
		if name != err.Suggestion && name != err.HistoricalName {
			m.options = append(m.options, remapOption{
				label:       fmt.Sprintf("Map to: %s", name),
				action:      RemapActionMap,
				targetName:  name,
				description: "Use this input instead",
			})
		}
	}

	m.selected = 0
}

// Update handles input for the remap modal.
func (m *RemapModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.options)-1 {
				m.selected++
			}
		case key.Matches(msg, m.keys.Enter):
			if m.selected < len(m.options) {
				opt := m.options[m.selected]
				err := m.errors[m.currentErrorIdx]

				decision := RemapDecision{
					OriginalName: err.HistoricalName,
					Action:       opt.action,
					NewName:      opt.targetName,
				}
				m.decisions = append(m.decisions, decision)

				// Move to next error or finish
				m.currentErrorIdx++
				if m.currentErrorIdx >= len(m.errors) {
					m.done = true
					return m, func() tea.Msg {
						return RemapResultMsg{Decisions: m.decisions}
					}
				}

				// Build options for next error
				m.buildOptions()
			}
			return m, nil
		case key.Matches(msg, m.keys.Escape):
			m.cancelled = true
			m.done = true
			return m, nil
		}
	}
	return m, nil
}

// View renders the remap modal.
func (m *RemapModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render("Configuration Remapping Wizard"))
	s.WriteString("\n\n")

	if m.currentErrorIdx >= len(m.errors) {
		s.WriteString(ui.SubtitleStyle.Render("All errors resolved"))
		return s.String()
	}

	err := m.errors[m.currentErrorIdx]
	progress := fmt.Sprintf("Error %d of %d", m.currentErrorIdx+1, len(m.errors))
	s.WriteString(ui.SubtitleStyle.Render(progress))
	s.WriteString("\n\n")

	// Show error details
	s.WriteString(ui.NormalStyle.Render("Historical input: "))
	s.WriteString(ui.SelectedStyle.Render(err.HistoricalName))
	s.WriteString("\n")

	s.WriteString(ui.NormalStyle.Render("Value: "))
	if err.HistoricalValue == "" {
		s.WriteString(ui.TableItalicStyle.Render(`("")`))
	} else {
		s.WriteString(ui.NormalStyle.Render(err.HistoricalValue))
	}
	s.WriteString("\n")

	s.WriteString(ui.NormalStyle.Render("Status: "))
	statusText := getStatusText(err.Status)
	s.WriteString(ui.TableItalicStyle.Render(statusText))
	s.WriteString("\n\n")

	// Show options
	s.WriteString(ui.SubtitleStyle.Render("Choose action:"))
	s.WriteString("\n\n")

	for i, opt := range m.options {
		cursor := "  "
		style := ui.NormalStyle
		if i == m.selected {
			cursor = "> "
			style = ui.SelectedStyle
		}
		s.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, opt.label)))
		s.WriteString("\n")
		if opt.description != "" {
			descStyle := ui.SubtitleStyle
			if i == m.selected {
				descStyle = ui.TableItalicStyle
			}
			s.WriteString("    ")
			s.WriteString(descStyle.Render(opt.description))
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")
	s.WriteString(ui.HelpStyle.Render("[↑↓/j/k] navigate  [enter] confirm  [esc] cancel"))

	return s.String()
}

// getStatusText returns a human-readable status message.
func getStatusText(status validation.ValidationStatus) string {
	switch status {
	case validation.StatusMissing:
		return "Input name no longer exists"
	case validation.StatusTypeChanged:
		return "Input type has changed"
	case validation.StatusOptionsChanged:
		return "Value not in valid options"
	default:
		return "Unknown error"
	}
}

// IsDone returns true if the modal is finished.
func (m *RemapModal) IsDone() bool {
	return m.done
}

// Result returns the remapping decisions.
func (m *RemapModal) Result() any {
	if m.cancelled {
		return nil
	}
	return m.decisions
}

// RemapResultMsg is sent when remapping is complete.
type RemapResultMsg struct {
	Decisions []RemapDecision
}
