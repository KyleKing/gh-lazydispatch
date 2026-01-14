package modal

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/gh-wfr/internal/ui"
)

// Context represents a modal that can be pushed onto the stack.
type Context interface {
	Update(msg tea.Msg) (Context, tea.Cmd)
	View() string
	IsDone() bool
	Result() any
}

// Stack manages a stack of modal contexts.
type Stack struct {
	contexts []Context
	width    int
	height   int
}

// NewStack creates a new empty modal stack.
func NewStack() *Stack {
	return &Stack{
		contexts: make([]Context, 0),
	}
}

// SetSize updates the dimensions for modal rendering.
func (s *Stack) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// Push adds a context to the top of the stack.
func (s *Stack) Push(ctx Context) {
	s.contexts = append(s.contexts, ctx)
}

// Pop removes and returns the top context.
func (s *Stack) Pop() Context {
	if len(s.contexts) == 0 {
		return nil
	}
	ctx := s.contexts[len(s.contexts)-1]
	s.contexts = s.contexts[:len(s.contexts)-1]
	return ctx
}

// Current returns the top context without removing it.
func (s *Stack) Current() Context {
	if len(s.contexts) == 0 {
		return nil
	}
	return s.contexts[len(s.contexts)-1]
}

// HasActive returns true if there's at least one modal on the stack.
func (s *Stack) HasActive() bool {
	return len(s.contexts) > 0
}

// Clear removes all contexts from the stack.
func (s *Stack) Clear() {
	s.contexts = s.contexts[:0]
}

// Update processes a message for the current modal.
func (s *Stack) Update(msg tea.Msg) tea.Cmd {
	if !s.HasActive() {
		return nil
	}

	ctx := s.Current()
	newCtx, cmd := ctx.Update(msg)
	s.contexts[len(s.contexts)-1] = newCtx

	if newCtx.IsDone() {
		s.Pop()
	}

	return cmd
}

// Render renders the modal overlay on top of the background.
func (s *Stack) Render(background string) string {
	if !s.HasActive() {
		return background
	}

	modalView := s.Current().View()
	return placeCenter(background, modalView, s.width, s.height)
}

func placeCenter(background, modal string, width, height int) string {
	modalStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(ui.PrimaryColor).
		Padding(2, 3).
		Background(ui.ModalBgColor)

	styledModal := modalStyle.Render(modal)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, styledModal, lipgloss.WithWhitespaceChars(" "), lipgloss.WithWhitespaceForeground(lipgloss.NoColor{}))
}

// ModalClosedMsg is sent when a modal is closed.
type ModalClosedMsg struct {
	Result any
}
