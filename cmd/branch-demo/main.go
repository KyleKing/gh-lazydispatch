package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/gh-lazydispatch/internal/git"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
)

type model struct {
	branchModal *modal.SimpleBranchModal
	width       int
	height      int
	result      string
	done        bool
	errorMsg    string
}

func initialModel() model {
	ctx := context.Background()

	// Fetch real branches
	branches, err := git.FetchBranches(ctx)
	if err != nil {
		branches = []string{"main", "master", "develop", "feature-1", "feature-2", "bugfix-1"}
	}

	// Get default branch
	defaultBranch := git.GetDefaultBranch(ctx)

	// Get current branch (if in git repo)
	current := "develop" // For demo purposes

	branchModal := modal.NewSimpleBranchModal("Select Branch (Demo)", branches, current, defaultBranch)
	// Set a reasonable default size - will update on WindowSizeMsg
	branchModal.SetSize(80, 30)

	return model{
		branchModal: branchModal,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.branchModal.SetSize(msg.Width, msg.Height)

		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" || (msg.String() == "q" && m.done) {
			return m, tea.Quit
		}

	case modal.BranchResultMsg:
		m.result = msg.Value
		m.done = true

		return m, nil
	}

	if !m.done {
		ctx, cmd := m.branchModal.Update(msg)
		m.branchModal = ctx.(*modal.SimpleBranchModal)

		if m.branchModal.IsDone() {
			result := m.branchModal.Result()
			if result != nil {
				m.result = result.(string)
			}

			m.done = true
		}

		return m, cmd
	}

	return m, nil
}

func (m model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	var content string

	if m.done {
		if m.result != "" {
			content = fmt.Sprintf("Selected branch: %s\n\nPress 'q' to quit.", m.result)
		} else {
			content = "Cancelled.\n\nPress 'q' to quit."
		}
	} else {
		// Show debug info
		debugInfo := fmt.Sprintf("Terminal: %dx%d\n", m.width, m.height)
		modalView := m.branchModal.View()
		content = debugInfo + "\n" + modalView
	}

	v := tea.NewView(content)
	v.AltScreen = true

	return v
}

func main() {
	p := tea.NewProgram(initialModel())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
