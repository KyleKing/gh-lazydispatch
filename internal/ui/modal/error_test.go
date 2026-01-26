package modal

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestErrorModal_Display(t *testing.T) {
	modal := NewErrorModal("Test Error", "Something went wrong")

	if modal.IsDone() {
		t.Error("modal should not be done initially")
	}

	view := modal.View()
	if !strings.Contains(view, "Test Error") {
		t.Error("view should contain title")
	}

	if !strings.Contains(view, "Something went wrong") {
		t.Error("view should contain message")
	}
}

func TestErrorModal_Dismiss(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"esc key", "esc"},
		{"q key", "q"},
		{"enter key", "enter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modal := NewErrorModal("Error", "Message")

			_, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			switch tt.key {
			case "esc":
				_, _ = modal.Update(tea.KeyMsg{Type: tea.KeyEsc})
			case "enter":
				_, _ = modal.Update(tea.KeyMsg{Type: tea.KeyEnter})
			}

			if !modal.IsDone() {
				t.Errorf("modal should be done after %s key", tt.key)
			}
		})
	}
}

func TestErrorModal_Result(t *testing.T) {
	modal := NewErrorModal("Error", "Message")

	result := modal.Result()
	if result != nil {
		t.Error("result should be nil for error modal")
	}
}

func TestErrorModal_MultilineMessage(t *testing.T) {
	modal := NewErrorModal("Error", "Line 1\nLine 2\nLine 3")

	view := modal.View()
	if !strings.Contains(view, "Line 1") {
		t.Error("view should contain first line")
	}

	if !strings.Contains(view, "Line 3") {
		t.Error("view should contain last line")
	}
}
