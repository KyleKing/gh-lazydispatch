package panes

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// WorkflowItem represents a workflow in the list.
type WorkflowItem struct {
	workflow workflow.File
}

// Title returns the workflow name, falling back to its filename.
func (i WorkflowItem) Title() string {
	if i.workflow.Name != "" {
		return i.workflow.Name
	}

	return i.workflow.Filename
}

// Description returns the workflow filename when a name is present.
func (i WorkflowItem) Description() string {
	if i.workflow.Name != "" {
		return i.workflow.Filename
	}

	return ""
}

// FilterValue returns the value used for fuzzy filtering.
func (i WorkflowItem) FilterValue() string {
	return i.workflow.Name + " " + i.workflow.Filename
}

// Workflow returns the underlying workflow file.
func (i WorkflowItem) Workflow() workflow.File {
	return i.workflow
}

// WorkflowModel manages the workflow list pane.
type WorkflowModel struct {
	list    list.Model
	focused bool
	width   int
	height  int
}

// NewWorkflowModel creates a new workflow pane model.
func NewWorkflowModel(workflows []workflow.File) WorkflowModel {
	items := make([]list.Item, len(workflows))
	for i, wf := range workflows {
		items[i] = WorkflowItem{workflow: wf}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = ui.SelectedStyle
	delegate.Styles.SelectedDesc = ui.SubtitleStyle
	delegate.Styles.NormalTitle = ui.NormalStyle
	delegate.Styles.NormalDesc = ui.SubtitleStyle

	l := list.New(items, delegate, 0, 0)
	l.Title = "Workflows"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.Styles.Title = ui.TitleStyle

	return WorkflowModel{list: l}
}

// workflowListChrome is the space reserved for the pane's border and title on each axis.
const workflowListChrome = 6

// SetSize updates the pane dimensions.
func (m *WorkflowModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width-workflowListChrome, height-workflowListChrome)
}

// SetFocused updates the focus state.
func (m *WorkflowModel) SetFocused(focused bool) {
	m.focused = focused
}

// Update handles messages for the workflow pane.
func (m WorkflowModel) Update(msg tea.Msg) (WorkflowModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	return m, cmd
}

// View renders the workflow pane.
func (m WorkflowModel) View() string {
	style := ui.PaneStyle(m.width, m.height, m.focused)
	return style.Render(m.list.View())
}

// SelectedWorkflow returns the currently selected workflow.
func (m WorkflowModel) SelectedWorkflow() *workflow.File {
	item := m.list.SelectedItem()
	if item == nil {
		return nil
	}

	wi, ok := item.(WorkflowItem)
	if !ok {
		return nil
	}

	wf := wi.Workflow()

	return &wf
}

// SelectedIndex returns the index of the selected workflow.
func (m WorkflowModel) SelectedIndex() int {
	return m.list.Index()
}

// WorkflowSelectedMsg is sent when a workflow is selected.
type WorkflowSelectedMsg struct {
	Workflow workflow.File
	Index    int
}

// HandleSelect processes a selection and returns a message.
func (m WorkflowModel) HandleSelect() tea.Cmd {
	wf := m.SelectedWorkflow()
	if wf == nil {
		return nil
	}

	return func() tea.Msg {
		return WorkflowSelectedMsg{Workflow: *wf, Index: m.SelectedIndex()}
	}
}
