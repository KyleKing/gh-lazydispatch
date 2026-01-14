package modal

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStack_PushPop(t *testing.T) {
	stack := NewStack()

	if stack.HasActive() {
		t.Error("expected empty stack")
	}

	modal := NewSelectModal("Test", []string{"a", "b"}, "a")
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

	modal1 := NewSelectModal("First", []string{"a"}, "a")
	modal2 := NewSelectModal("Second", []string{"b"}, "b")

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
	modal := NewSelectModal("Test", []string{"a", "b", "c"}, "a")

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
	modal := NewSelectModal("Test", []string{"a", "b", "c"}, "b")

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
	modal := NewSelectModal("Test", []string{"a", "b"}, "a")

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
	modal := NewInputModal("Title", "Description", "default", "string", "initial")

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
	modal := NewInputModal("Title", "", "", "", "value")

	esc := tea.KeyMsg{Type: tea.KeyEscape}
	modal.Update(esc)

	if !modal.IsDone() {
		t.Error("expected modal to be done after escape")
	}
}

func TestConfirmModal_Navigation(t *testing.T) {
	modal := NewConfirmModal("Confirm?", "", true)

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
	modal := NewConfirmModal("Confirm?", "", false)

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
	modal := NewConfirmModal("Confirm?", "", true)

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
	modal := NewConfirmModal("Delete file?", "This cannot be undone", true)

	view := modal.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}
