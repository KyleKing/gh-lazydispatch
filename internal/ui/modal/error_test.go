package modal_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
)

func TestErrorModal_Display(t *testing.T) {
	t.Parallel()

	em := modal.NewErrorModal("Test Error", "Something went wrong")

	if em.IsDone() {
		t.Error("modal should not be done initially")
	}

	view := em.View()
	if !strings.Contains(view, "Test Error") {
		t.Error("view should contain title")
	}

	if !strings.Contains(view, "Something went wrong") {
		t.Error("view should contain message")
	}
}

func TestErrorModal_Dismiss(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			em := modal.NewErrorModal("Error", "Message")

			switch tt.key {
			case "esc":
				_, _ = em.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
			case "q":
				_, _ = em.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
			case "enter":
				_, _ = em.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			}

			if !em.IsDone() {
				t.Errorf("modal should be done after %s key", tt.key)
			}
		})
	}
}

func TestErrorModal_Result(t *testing.T) {
	t.Parallel()

	em := modal.NewErrorModal("Error", "Message")

	result := em.Result()
	if result != nil {
		t.Error("result should be nil for error modal")
	}
}

func TestErrorModal_MultilineMessage(t *testing.T) {
	t.Parallel()

	em := modal.NewErrorModal("Error", "Line 1\nLine 2\nLine 3")

	view := em.View()
	if !strings.Contains(view, "Line 1") {
		t.Error("view should contain first line")
	}

	if !strings.Contains(view, "Line 3") {
		t.Error("view should contain last line")
	}
}
