package app

import (
	"sort"
	"strings"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

func (m Model) currentHistoryEntries() []frecency.HistoryEntry {
	if m.history == nil {
		return nil
	}
	var workflowFilter string
	if m.selectedWorkflow >= 0 && m.selectedWorkflow < len(m.workflows) {
		workflowFilter = m.workflows[m.selectedWorkflow].Filename
	}
	return m.history.TopForRepo(m.repo, workflowFilter, MaxHistoryEntries)
}

// SelectedWorkflow returns the currently selected workflow.
func (m Model) SelectedWorkflow() *workflow.WorkflowFile {
	if m.selectedWorkflow < 0 || m.selectedWorkflow >= len(m.workflows) {
		return nil
	}
	return &m.workflows[m.selectedWorkflow]
}

func (m *Model) initializeInputs(wf workflow.WorkflowFile) {
	m.inputs = make(map[string]string)
	m.inputOrder = nil
	for name, input := range wf.GetInputs() {
		m.inputs[name] = input.Default
		m.inputOrder = append(m.inputOrder, name)
	}
	sort.Strings(m.inputOrder)
	m.filteredInputs = m.inputOrder
	m.filterText = ""
	m.selectedInput = -1
	m.viewMode = WorkflowListMode
	m.syncHistoryEntries()
}

func (m *Model) syncHistoryEntries() {
	entries := m.currentHistoryEntries()
	var workflowFilter string
	if m.selectedWorkflow >= 0 && m.selectedWorkflow < len(m.workflows) {
		workflowFilter = m.workflows[m.selectedWorkflow].Filename
	}
	m.rightPanel.SetHistoryEntries(entries, workflowFilter)
}

func (m Model) getSelectedInputName() string {
	if len(m.filteredInputs) == 0 {
		return ""
	}
	if m.selectedInput < 0 || m.selectedInput >= len(m.filteredInputs) {
		return ""
	}
	return m.filteredInputs[m.selectedInput]
}

func _padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}

func _formatRowNumber(index int) string {
	displayIdx := index
	if displayIdx <= 9 {
		return string(rune('0' + displayIdx))
	}
	return " "
}

func _contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func _wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0
	for i, word := range words {
		if i > 0 && lineLen+1+len(word) > width {
			result.WriteString("\n")
			lineLen = 0
		} else if i > 0 {
			result.WriteString(" ")
			lineLen++
		}
		result.WriteString(word)
		lineLen += len(word)
	}
	return result.String()
}
