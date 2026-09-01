package modal

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func typeInto(t *testing.T, m *SimpleBranchModal, text string) *SimpleBranchModal {
	t.Helper()

	for _, r := range text {
		ctx, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})

		next, ok := ctx.(*SimpleBranchModal)
		if !ok {
			t.Fatalf("Update returned %T, want *SimpleBranchModal", ctx)
		}

		m = next
	}

	return m
}

func TestBranchPinning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		current       string
		defaultBranch string
		wantFirst     string
		wantSecond    string
		branches      []string
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
			wantSecond:    "main",
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
			t.Parallel()

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

// TestSimpleBranchModal_OpensOnTheCurrentBranch guards the case a user hits
// every time they open the modal without meaning to change anything: enter
// must return the branch they were already on.
func TestSimpleBranchModal_OpensOnTheCurrentBranch(t *testing.T) {
	t.Parallel()

	m := NewSimpleBranchModal("Select Branch", []string{"main", "develop", "feature"}, "feature", "main")

	ctx, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !ctx.IsDone() {
		t.Fatal("enter did not close the modal")
	}

	if got := ctx.Result(); got != "feature" {
		t.Errorf("result is %v, want the current branch", got)
	}

	msg, ok := cmd().(BranchResultMsg)
	if !ok || msg.Value != "feature" {
		t.Errorf("command produced %#v, want BranchResultMsg for the current branch", cmd())
	}
}

// TestSimpleBranchModal_TypingFiltersAcrossEveryBranch checks that filtering
// searches the whole branch list rather than only the pinned head of it.
func TestSimpleBranchModal_TypingFiltersAcrossEveryBranch(t *testing.T) {
	t.Parallel()

	branches := []string{"main", "develop", "feature/login", "feature/logout", "hotfix"}
	m := typeInto(t, NewSimpleBranchModal("Select Branch", branches, "main", "main"), "logout")

	if !m.filtering {
		t.Fatal("typing a printable character did not start filtering")
	}

	if len(m.filteredBranches) != 1 || m.filteredBranches[0] != "feature/logout" {
		t.Fatalf("filter matched %v, want only feature/logout", m.filteredBranches)
	}

	ctx, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	ctx, _ = ctx.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := ctx.Result(); got != "feature/logout" {
		t.Errorf("result is %v, want the filtered branch", got)
	}
}

// TestSimpleBranchModal_EscapeClearsTheFilterBeforeClosing keeps escape from
// discarding the whole selection when the user only wants to widen the filter.
func TestSimpleBranchModal_EscapeClearsTheFilterBeforeClosing(t *testing.T) {
	t.Parallel()

	branches := []string{"main", "develop", "feature"}
	m := typeInto(t, NewSimpleBranchModal("Select Branch", branches, "main", "main"), "dev")

	ctx, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	m, ok := ctx.(*SimpleBranchModal)
	if !ok {
		t.Fatalf("Update returned %T, want *SimpleBranchModal", ctx)
	}

	if m.done {
		t.Fatal("escape closed the modal instead of clearing the filter")
	}

	if len(m.filteredBranches) != len(branches) {
		t.Errorf("filter left %d branches, want all %d back", len(m.filteredBranches), len(branches))
	}

	ctx, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	ctx, _ = ctx.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if !ctx.IsDone() {
		t.Error("escape on an empty filter did not close the modal")
	}

	if got := ctx.Result(); got != "" {
		t.Errorf("canceling returned %v, want no branch", got)
	}
}

// TestSimpleBranchModal_ScrollsToKeepTheSelectionVisible pins the windowing
// arithmetic: a list longer than the modal must scroll rather than run the
// cursor off the bottom.
func TestSimpleBranchModal_ScrollsToKeepTheSelectionVisible(t *testing.T) {
	t.Parallel()

	branches := make([]string, 40)
	for i := range branches {
		branches[i] = "branch-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
	}

	m := NewSimpleBranchModal("Select Branch", branches, branches[0], branches[0])
	m.SetSize(80, 30)

	for range len(branches) - 1 {
		ctx, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})

		next, ok := ctx.(*SimpleBranchModal)
		if !ok {
			t.Fatalf("Update returned %T, want *SimpleBranchModal", ctx)
		}

		m = next
	}

	if m.selected != len(branches)-1 {
		t.Fatalf("selection stopped at %d, want the last branch %d", m.selected, len(branches)-1)
	}

	if m.selected < m.scrollOffset || m.selected >= m.scrollOffset+m.maxHeight {
		t.Errorf("selection %d is outside the visible window [%d, %d)",
			m.selected, m.scrollOffset, m.scrollOffset+m.maxHeight)
	}

	if view := m.View(); !strings.Contains(view, branches[len(branches)-1]) {
		t.Error("the selected branch is not in the rendered view")
	}
}

func TestSimpleBranchModal_ReportsAnEmptyFilter(t *testing.T) {
	t.Parallel()

	m := typeInto(t, NewSimpleBranchModal("Select Branch", []string{"main", "develop"}, "main", "main"), "zzz")

	if view := m.View(); !strings.Contains(view, "No branches found") {
		t.Errorf("a filter matching nothing renders no explanation:\n%s", view)
	}

	ctx, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	ctx, cmd := ctx.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := ctx.Result(); got != "" {
		t.Errorf("selecting from an empty list returned %v, want nothing", got)
	}

	if msg, ok := cmd().(BranchResultMsg); !ok || msg.Value != "" {
		t.Errorf("command produced %#v, want an empty BranchResultMsg", cmd())
	}
}
