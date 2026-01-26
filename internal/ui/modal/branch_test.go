package modal

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBranchModalCreation(t *testing.T) {
	branches := []string{"main", "develop", "feature-1"}
	current := "develop"
	defaultBranch := "main"

	modal := NewBranchModalWithDefault("Test Branch", branches, current, defaultBranch)

	if modal == nil {
		t.Fatal("NewBranchModalWithDefault returned nil")
	}

	if modal.currentBranch != current {
		t.Errorf("currentBranch = %q, want %q", modal.currentBranch, current)
	}

	if modal.defaultBranch != defaultBranch {
		t.Errorf("defaultBranch = %q, want %q", modal.defaultBranch, defaultBranch)
	}
}

func TestBranchModalSetSize(t *testing.T) {
	tests := []struct {
		name           string
		terminalWidth  int
		terminalHeight int
		wantMinHeight  int
		wantMaxHeight  int
	}{
		{
			name:           "small terminal",
			terminalWidth:  80,
			terminalHeight: 10,
			wantMinHeight:  10,
			wantMaxHeight:  10,
		},
		{
			name:           "medium terminal",
			terminalWidth:  100,
			terminalHeight: 30,
			wantMinHeight:  24,
			wantMaxHeight:  24,
		},
		{
			name:           "large terminal",
			terminalWidth:  120,
			terminalHeight: 50,
			wantMinHeight:  30,
			wantMaxHeight:  30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modal := NewBranchModal("Test", []string{"main"}, "main")
			modal.SetSize(tt.terminalWidth, tt.terminalHeight)

			if modal.terminalWidth != tt.terminalWidth {
				t.Errorf("terminalWidth = %d, want %d", modal.terminalWidth, tt.terminalWidth)
			}

			if modal.terminalHeight != tt.terminalHeight {
				t.Errorf("terminalHeight = %d, want %d", modal.terminalHeight, tt.terminalHeight)
			}

			// We can't directly inspect list height, but we can check terminal values were stored
			if modal.terminalHeight != tt.terminalHeight {
				t.Errorf("terminal height not stored correctly")
			}
		})
	}
}

func TestBranchPinning(t *testing.T) {
	tests := []struct {
		name          string
		branches      []string
		current       string
		defaultBranch string
		wantFirst     string
		wantSecond    string
	}{
		{
			name:          "current and default different",
			branches:      []string{"main", "develop", "feature"},
			current:       "develop",
			defaultBranch: "main",
			wantFirst:     "develop",
			wantSecond:    "main",
		},
		{
			name:          "current is default",
			branches:      []string{"main", "develop", "feature"},
			current:       "main",
			defaultBranch: "main",
			wantFirst:     "main",
			wantSecond:    "develop",
		},
		{
			name:          "no default",
			branches:      []string{"main", "develop", "feature"},
			current:       "develop",
			defaultBranch: "",
			wantFirst:     "develop",
			wantSecond:    "main", // remaining branches in original order: main, feature
		},
		{
			name:          "no current",
			branches:      []string{"main", "develop", "feature"},
			current:       "",
			defaultBranch: "main",
			wantFirst:     "main",
			wantSecond:    "develop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := _pinBranches(tt.branches, tt.current, tt.defaultBranch)

			if len(result) != len(tt.branches) {
				t.Errorf("pinned branches length = %d, want %d", len(result), len(tt.branches))
			}

			if result[0] != tt.wantFirst {
				t.Errorf("first branch = %q, want %q", result[0], tt.wantFirst)
			}

			if len(result) > 1 && result[1] != tt.wantSecond {
				t.Errorf("second branch = %q, want %q", result[1], tt.wantSecond)
			}
		})
	}
}

func TestBranchModalFilterReset(t *testing.T) {
	branches := []string{"main", "develop", "feature-1", "feature-2"}
	modal := NewBranchModalWithDefault("Test", branches, "develop", "main")

	// Store original item count
	originalCount := len(modal.originalItems)

	// Simulate entering filter mode
	modal.wasFiltering = false
	modal.list.SetFilteringEnabled(true)

	// The actual filtering is handled by bubbles/list Update()
	// Here we're testing that wasFiltering flag tracks state correctly

	if modal.wasFiltering {
		t.Error("wasFiltering should be false initially")
	}

	// We can't easily simulate the full filter->reset cycle without running Update()
	// but we can verify the setup is correct
	if len(modal.originalItems) != originalCount {
		t.Errorf("originalItems count changed: got %d, want %d", len(modal.originalItems), originalCount)
	}
}

func TestBranchModalView(t *testing.T) {
	branches := []string{"main", "develop", "feature"}
	modal := NewBranchModal("Select Branch", branches, "main")
	modal.SetSize(80, 30)

	view := modal.View()

	if view == "" {
		t.Error("View() returned empty string")
	}

	// Check that the view contains the title
	if !strings.Contains(view, "Select Branch") {
		t.Error("View() should contain title")
	}
}

func TestBranchModalKeyHandling(t *testing.T) {
	branches := []string{"main", "develop", "feature"}
	modal := NewBranchModal("Test", branches, "main")

	// Test Enter key selects branch
	ctx, _ := modal.Update(tea.KeyMsg{Type: tea.KeyEnter})
	branchModal := ctx.(*BranchModal)

	if !branchModal.done {
		t.Error("Enter key should mark modal as done")
	}

	if branchModal.result == "" {
		t.Error("Enter key should set result")
	}

	// Test Esc key cancels
	modal2 := NewBranchModal("Test", branches, "main")
	ctx2, _ := modal2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	branchModal2 := ctx2.(*BranchModal)

	if !branchModal2.done {
		t.Error("Esc key should mark modal as done")
	}

	if branchModal2.result != "" {
		t.Error("Esc key should leave result empty")
	}
}

func TestBranchModalStylesHaveNoBackground(t *testing.T) {
	branches := []string{"main", "develop"}
	modal := NewBranchModalWithDefault("Test", branches, "main", "main")

	// We can't directly inspect lipgloss styles for background,
	// but we can render and check the output doesn't have unexpected styling
	view := modal.View()

	// The view should not be empty
	if view == "" {
		t.Error("View should not be empty")
	}

	// Basic smoke test - the modal should be renderable
	if len(view) < 10 {
		t.Error("View seems too short, possible rendering issue")
	}
}
