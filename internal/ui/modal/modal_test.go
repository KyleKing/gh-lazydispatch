package modal

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
)

func TestStack_PushPop(t *testing.T) {
	stack := NewStack()

	if stack.HasActive() {
		t.Error("expected empty stack")
	}

	modal := NewSelectModal("Test", []string{"a", "b"}, "a", "a")
	stack.Push(modal)

	if !stack.HasActive() {
		t.Error("expected stack to have active modal")
	}

	popped := stack.Pop()
	if popped == nil {
		t.Error("expected non-nil modal")
	}

	if stack.HasActive() {
		t.Error("expected empty stack after pop")
	}
}

func TestStack_Current(t *testing.T) {
	stack := NewStack()

	if stack.Current() != nil {
		t.Error("expected nil current on empty stack")
	}

	modal1 := NewSelectModal("First", []string{"a"}, "a", "a")
	modal2 := NewSelectModal("Second", []string{"b"}, "b", "b")

	stack.Push(modal1)
	stack.Push(modal2)

	current := stack.Current()
	if current == nil {
		t.Fatal("expected non-nil current")
	}

	if selectModal, ok := current.(*SelectModal); ok {
		if selectModal.title != "Second" {
			t.Errorf("expected 'Second', got %q", selectModal.title)
		}
	}
}

func TestSelectModal_Navigation(t *testing.T) {
	modal := NewSelectModal("Test", []string{"a", "b", "c"}, "a", "a")

	if modal.selected != 0 {
		t.Errorf("expected selected 0, got %d", modal.selected)
	}

	down := tea.KeyMsg{Type: tea.KeyDown}
	modal.Update(down)

	if modal.selected != 1 {
		t.Errorf("expected selected 1 after down, got %d", modal.selected)
	}

	up := tea.KeyMsg{Type: tea.KeyUp}
	modal.Update(up)

	if modal.selected != 0 {
		t.Errorf("expected selected 0 after up, got %d", modal.selected)
	}
}

func TestSelectModal_Select(t *testing.T) {
	modal := NewSelectModal("Test", []string{"a", "b", "c"}, "b", "a")

	if modal.selected != 1 {
		t.Errorf("expected initial selection 1 (current='b'), got %d", modal.selected)
	}

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	modal.Update(enter)

	if !modal.IsDone() {
		t.Error("expected modal to be done after enter")
	}

	if modal.Result() != "b" {
		t.Errorf("expected result 'b', got %v", modal.Result())
	}
}

func TestSelectModal_Escape(t *testing.T) {
	modal := NewSelectModal("Test", []string{"a", "b"}, "a", "a")

	esc := tea.KeyMsg{Type: tea.KeyEscape}
	modal.Update(esc)

	if !modal.IsDone() {
		t.Error("expected modal to be done after escape")
	}

	if modal.Result() != "" {
		t.Errorf("expected empty result after escape, got %v", modal.Result())
	}
}

func TestInputModal_Enter(t *testing.T) {
	modal := NewInputModal("Title", "Description", "default", "string", "initial", nil, nil)

	if modal.input.Value() != "initial" {
		t.Errorf("expected initial value 'initial', got %q", modal.input.Value())
	}

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	modal.Update(enter)

	if !modal.IsDone() {
		t.Error("expected modal to be done after enter")
	}

	if modal.Result() != "initial" {
		t.Errorf("expected result 'initial', got %v", modal.Result())
	}
}

func TestInputModal_Escape(t *testing.T) {
	modal := NewInputModal("Title", "", "", "", "value", nil, nil)

	esc := tea.KeyMsg{Type: tea.KeyEscape}
	modal.Update(esc)

	if !modal.IsDone() {
		t.Error("expected modal to be done after escape")
	}
}

func TestConfirmModal_Navigation(t *testing.T) {
	modal := NewConfirmModal("Confirm?", "", true, true)

	if !modal.selected {
		t.Error("expected selected to be true initially")
	}

	right := tea.KeyMsg{Type: tea.KeyRight}
	modal.Update(right)

	if modal.selected {
		t.Error("expected selected to be false after right")
	}

	left := tea.KeyMsg{Type: tea.KeyLeft}
	modal.Update(left)

	if !modal.selected {
		t.Error("expected selected to be true after left")
	}
}

func TestConfirmModal_QuickKeys(t *testing.T) {
	modal := NewConfirmModal("Confirm?", "", false, false)

	y := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	modal.Update(y)

	if !modal.IsDone() {
		t.Error("expected modal to be done after 'y'")
	}

	if modal.Result() != true {
		t.Errorf("expected result true, got %v", modal.Result())
	}
}

func TestConfirmModal_QuickNo(t *testing.T) {
	modal := NewConfirmModal("Confirm?", "", true, true)

	n := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	modal.Update(n)

	if !modal.IsDone() {
		t.Error("expected modal to be done after 'n'")
	}

	if modal.Result() != false {
		t.Errorf("expected result false, got %v", modal.Result())
	}
}

func TestConfirmModal_View(t *testing.T) {
	modal := NewConfirmModal("Delete file?", "This cannot be undone", true, true)

	view := modal.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestRunConfirmModal_Confirm(t *testing.T) {
	cfg := runner.RunConfig{
		Workflow: "test.yml",
		Branch:   "main",
		Inputs:   map[string]string{"env": "prod"},
		Watch:    false,
	}

	modal := NewRunConfirmModal(cfg)

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	modal.Update(enter)

	if !modal.IsDone() {
		t.Error("expected modal to be done after enter")
	}

	result, ok := modal.Result().(RunConfirmResultMsg)
	if !ok {
		t.Fatal("expected RunConfirmResultMsg")
	}
	if !result.Confirmed {
		t.Error("expected confirmed=true")
	}
	if result.Config.Workflow != "test.yml" {
		t.Errorf("expected workflow=test.yml, got %q", result.Config.Workflow)
	}
}

func TestRunConfirmModal_Cancel(t *testing.T) {
	cfg := runner.RunConfig{Workflow: "test.yml"}
	modal := NewRunConfirmModal(cfg)

	esc := tea.KeyMsg{Type: tea.KeyEscape}
	modal.Update(esc)

	if !modal.IsDone() {
		t.Error("expected modal to be done after escape")
	}

	result, ok := modal.Result().(RunConfirmResultMsg)
	if !ok {
		t.Fatal("expected RunConfirmResultMsg")
	}
	if result.Confirmed {
		t.Error("expected confirmed=false after escape")
	}
}

func TestFilterModal_ApplyFilter(t *testing.T) {
	items := []string{"environment", "debug", "verbose"}
	modal := NewFilterModal("Filter", items, "")

	enterE := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	modal.Update(enterE)

	enterN := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	modal.Update(enterN)

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := modal.Update(enter)

	if !modal.IsDone() {
		t.Error("expected modal to be done after enter")
	}

	if cmd == nil {
		t.Fatal("expected command")
	}

	msg := cmd()
	result, ok := msg.(FilterResultMsg)
	if !ok {
		t.Fatalf("expected FilterResultMsg, got %T", msg)
	}
	if result.Cancelled {
		t.Error("expected cancelled=false")
	}
	if result.Value != "en" {
		t.Errorf("expected value='en', got %q", result.Value)
	}
}

func TestFilterModal_Cancel(t *testing.T) {
	items := []string{"a", "b"}
	modal := NewFilterModal("Filter", items, "initial")

	esc := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := modal.Update(esc)

	if !modal.IsDone() {
		t.Error("expected modal to be done after escape")
	}

	if cmd == nil {
		t.Fatal("expected command")
	}

	msg := cmd()
	result, ok := msg.(FilterResultMsg)
	if !ok {
		t.Fatalf("expected FilterResultMsg, got %T", msg)
	}
	if !result.Cancelled {
		t.Error("expected cancelled=true")
	}
}

func TestResetModal_Confirm(t *testing.T) {
	diffs := []ResetDiff{
		{Name: "env", Current: "prod", Default: "staging"},
		{Name: "debug", Current: "true", Default: "false"},
	}
	modal := NewResetModal(diffs)

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := modal.Update(enter)

	if !modal.IsDone() {
		t.Error("expected modal to be done after enter")
	}

	if cmd == nil {
		t.Fatal("expected command")
	}

	msg := cmd()
	result, ok := msg.(ResetResultMsg)
	if !ok {
		t.Fatalf("expected ResetResultMsg, got %T", msg)
	}
	if !result.Confirmed {
		t.Error("expected confirmed=true")
	}
}

func TestResetModal_Cancel(t *testing.T) {
	diffs := []ResetDiff{{Name: "a", Current: "b", Default: "c"}}
	modal := NewResetModal(diffs)

	esc := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := modal.Update(esc)

	if !modal.IsDone() {
		t.Error("expected modal to be done after escape")
	}

	if cmd == nil {
		t.Fatal("expected command")
	}

	msg := cmd()
	result, ok := msg.(ResetResultMsg)
	if !ok {
		t.Fatalf("expected ResetResultMsg, got %T", msg)
	}
	if result.Confirmed {
		t.Error("expected confirmed=false after escape")
	}
}

func TestHelpModal(t *testing.T) {
	modal := NewHelpModal()

	if modal.IsDone() {
		t.Error("expected modal not done initially")
	}

	esc := tea.KeyMsg{Type: tea.KeyEscape}
	modal.Update(esc)

	if !modal.IsDone() {
		t.Error("expected modal to be done after escape")
	}
}

func TestValidationErrorModal_Override(t *testing.T) {
	errors := map[string][]string{
		"env": {"must not be empty"},
	}
	modal := NewValidationErrorModal(errors)

	c := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	_, cmd := modal.Update(c)

	if !modal.IsDone() {
		t.Error("expected modal to be done after 'c'")
	}

	if cmd == nil {
		t.Fatal("expected command")
	}

	msg := cmd()
	result, ok := msg.(ValidationErrorResultMsg)
	if !ok {
		t.Fatalf("expected ValidationErrorResultMsg, got %T", msg)
	}
	if !result.Override {
		t.Error("expected override=true")
	}
}

func TestValidationErrorModal_Cancel(t *testing.T) {
	errors := map[string][]string{"a": {"error"}}
	modal := NewValidationErrorModal(errors)

	esc := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := modal.Update(esc)

	if !modal.IsDone() {
		t.Error("expected modal to be done after escape")
	}

	if cmd != nil {
		t.Error("expected no command on escape")
	}

	result := modal.Result()
	resultMsg, ok := result.(ValidationErrorResultMsg)
	if ok && resultMsg.Override {
		t.Error("expected override=false after escape")
	}
}
