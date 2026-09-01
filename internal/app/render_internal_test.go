package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

func newRenderModel() Model {
	m := New(testWorkflows(), testHistory(), "owner/repo")
	m.branch = "main"
	m.ghClient = nil
	m.logManager = nil
	m.watcher = nil
	m.wfdConfig = nil

	// The chains pane is part of the left column, so the frames have to cover a
	// repository that configures some.
	m.chains.SetChains(map[string]config.Chain{
		"demo-pipeline": {
			Description: "build then deploy",
			Steps:       []config.ChainStep{{Workflow: "build.yml"}, {Workflow: "deploy.yml"}},
			Variables:   []config.ChainVariable{{Name: "env"}},
		},
	})

	return m
}

func applyMsg(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()

	updated, cmd := m.Update(msg)

	model, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want app.Model", updated)
	}

	return drainCmd(t, model, cmd)
}

// drainCmd executes returned commands and feeds resulting messages back into
// Update so modal result messages reach their handlers, mirroring the runtime
// message loop. Commands are invoked directly, which constructs messages
// without side effects (tea.ExecProcess only builds its message lazily).
func drainCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()

	for range 16 {
		if cmd == nil {
			return m
		}

		msg := cmd()
		if msg == nil {
			return m
		}

		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				m = drainCmd(t, m, c)
			}

			return m
		}

		updated, next := m.Update(msg)

		model, ok := updated.(Model)
		if !ok {
			t.Fatalf("Update returned %T, want app.Model", updated)
		}

		m = model
		cmd = next
	}

	t.Fatal("command chain did not settle after 16 iterations")

	return m
}

func resize(t *testing.T, m Model, width, height int) Model {
	t.Helper()

	return applyMsg(t, m, tea.WindowSizeMsg{Width: width, Height: height})
}

func pressRune(t *testing.T, m Model, code rune) Model {
	t.Helper()

	return applyMsg(t, m, tea.KeyPressMsg{Code: code, Text: string(code)})
}

func pressSpecial(t *testing.T, m Model, code rune) Model {
	t.Helper()

	return applyMsg(t, m, tea.KeyPressMsg{Code: code})
}

func assertFits(t *testing.T, content string, width, height int) {
	t.Helper()

	lines := strings.Split(content, "\n")
	if len(lines) > height {
		t.Errorf("content has %d lines, terminal height is %d", len(lines), height)
	}

	for i, line := range lines {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("line %d has visible width %d, terminal width is %d", i+1, w, width)
		}
	}
}

// assertBottomPaneCloses catches the failure a width check cannot: a pane whose
// content wraps one line too far loses its bottom border to MaxHeight rather
// than overflowing, so the frame still fits the terminal and still reads wrong.
func assertBottomPaneCloses(t *testing.T, content string, width, height int) {
	t.Helper()

	if width < MinTerminalWidth || height < MinTerminalHeight {
		return
	}

	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		t.Fatalf("the frame has %d lines", len(lines))
	}

	bottom := ansi.Strip(lines[len(lines)-2])
	if !strings.HasPrefix(bottom, "╰") && !strings.HasPrefix(bottom, "┗") {
		t.Errorf("the bottom-left pane did not close its border:\n%s", bottom)
	}
}

var renderSizes = []struct {
	name          string
	width, height int
}{
	{"tiny_40x10", 40, 10},
	{"small_80x24", 80, 24},
	{"standard_120x40", 120, 40},
	{"wide_160x50", 160, 50},
}

func TestViewAtSizes(t *testing.T) {
	t.Parallel()

	for _, tt := range renderSizes {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := resize(t, newRenderModel(), tt.width, tt.height)

			content := m.View().Content
			assertFits(t, content, tt.width, tt.height)
			assertBottomPaneCloses(t, content, tt.width, tt.height)
			golden.RequireEqual(t, content)
		})
	}
}

func TestModalViewsAtStandardSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		open func(t *testing.T, m Model) Model
	}{
		{"help", func(t *testing.T, m Model) Model {
			t.Helper()

			return pressRune(t, m, '?')
		}},
		{"filter", func(t *testing.T, m Model) Model {
			t.Helper()

			m.focused = PaneConfig

			return pressRune(t, m, '/')
		}},
		{"input_environment", func(t *testing.T, m Model) Model {
			t.Helper()

			m.focused = PaneConfig

			return pressRune(t, m, '1')
		}},
		{"run_confirm", func(t *testing.T, m Model) Model {
			t.Helper()

			m.focused = PaneConfig

			return pressSpecial(t, m, tea.KeyEnter)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := tt.open(t, resize(t, newRenderModel(), 120, 40))

			if !m.modalStack.HasActive() {
				t.Fatal("expected a modal to be active")
			}

			content := m.View().Content
			assertFits(t, content, 120, 40)
			golden.RequireEqual(t, content)
		})
	}
}

func TestHelpModalTinyTerminal(t *testing.T) {
	t.Parallel()

	m := pressRune(t, resize(t, newRenderModel(), 40, 10), '?')

	content := m.View().Content
	assertFits(t, content, 40, 10)
	golden.RequireEqual(t, content)
}

func TestDispatchFlowEndState(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneConfig

	m = pressRune(t, m, '1')
	if !m.modalStack.HasActive() {
		t.Fatal("expected input modal after pressing 1")
	}

	m = pressSpecial(t, m, tea.KeyDown)
	m = pressSpecial(t, m, tea.KeyEnter)

	if m.modalStack.HasActive() {
		t.Fatal("expected input modal to close after enter")
	}

	m.focused = PaneConfig
	m = pressSpecial(t, m, tea.KeyEnter)

	if !m.modalStack.HasActive() {
		t.Fatal("expected run confirm modal after enter on config pane")
	}

	m = pressRune(t, m, 'y')

	if m.modalStack.HasActive() {
		t.Fatal("expected run confirm modal to close after confirming")
	}

	entries := m.history.TopForRepo("owner/repo", "deploy.yml", 1)
	if len(entries) == 0 {
		t.Fatal("expected dispatch to be recorded in history")
	}

	if got := entries[0].Inputs["environment"]; got == "" {
		t.Errorf("expected environment input to be recorded, got %q", entries[0].Inputs)
	}

	content := m.View().Content
	assertFits(t, content, 120, 40)
	golden.RequireEqual(t, content)
}

// manyWorkflows is the shape of a real repository: irm has 26 dispatchable
// workflows, which is more rows than the left pane has at any supported height.
func manyWorkflows(n int) []workflow.File {
	files := make([]workflow.File, 0, n)
	for i := range n {
		files = append(files, workflow.File{
			Name:     fmt.Sprintf("Workflow %02d", i),
			Filename: fmt.Sprintf("wf-%02d.yml", i),
			On:       workflow.OnTrigger{Dispatch: &workflow.Dispatch{}},
		})
	}

	return files
}

func TestView_FitsWhenWorkflowsOutnumberTheRows(t *testing.T) {
	t.Parallel()

	const workflowCount = 26

	for _, tt := range renderSizes {
		for _, selected := range []int{-1, 0, workflowCount / 2, workflowCount - 1} {
			t.Run(fmt.Sprintf("%s/selected_%d", tt.name, selected), func(t *testing.T) {
				t.Parallel()

				m := newRenderModel()
				m.workflows = manyWorkflows(workflowCount)
				m.selectedWorkflow = selected
				m = resize(t, m, tt.width, tt.height)

				content := m.View().Content
				assertFits(t, content, tt.width, tt.height)

				if tt.height < MinTerminalHeight || selected < 0 {
					return
				}

				want := fmt.Sprintf("Workflow %02d", selected)
				if !strings.Contains(ansi.Strip(content), want) {
					t.Errorf("selected workflow %q is outside the drawn window", want)
				}
			})
		}
	}
}
