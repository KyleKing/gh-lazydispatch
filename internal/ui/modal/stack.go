package modal

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-lazydispatch/internal/ui"
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

	for _, ctx := range s.contexts {
		s.size(ctx)
	}
}

// Sizer is implemented by modals that render themselves against the terminal
// dimensions. Stack hands those out on Push and on every resize, so a modal
// never has to guess how much room it has.
type Sizer interface {
	SetSize(width, height int)
}

// Push adds a context to the top of the stack.
func (s *Stack) Push(ctx Context) {
	s.size(ctx)
	s.contexts = append(s.contexts, ctx)
}

// size hands a modal the room left inside the border and padding.
func (s *Stack) size(ctx Context) {
	sizer, ok := ctx.(Sizer)
	if !ok || s.width == 0 || s.height == 0 {
		return
	}

	sizer.SetSize(s.width-modalChromeHorizontal, s.height-modalChromeVertical)
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

// Find returns the first context on the stack that satisfies match, searching
// from the top down. A long-running operation uses it to reach the modal
// reporting on it, which need not be the modal the user is looking at.
func (s *Stack) Find(match func(Context) bool) Context {
	for i := len(s.contexts) - 1; i >= 0; i-- {
		if match(s.contexts[i]) {
			return s.contexts[i]
		}
	}

	return nil
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

const (
	modalPaddingVertical   = 2
	modalPaddingHorizontal = 3
)

// The border and padding a modal's content sits inside of.
const (
	modalChromeVertical   = 2 + 2*modalPaddingVertical
	modalChromeHorizontal = 2 + 2*modalPaddingHorizontal
)

func placeCenter(_, modal string, width, height int) string {
	modalStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(ui.PrimaryColor).
		Padding(modalPaddingVertical, modalPaddingHorizontal).
		Background(ui.ModalBgColor)

	styledModal := modalStyle.Render(clip(modal, width-modalChromeHorizontal, height-modalChromeVertical))

	return lipgloss.Place(
		width, height, lipgloss.Center, lipgloss.Center, styledModal, lipgloss.WithWhitespaceChars(" "),
	)
}

// elisionNotice replaces the lines a modal taller than the terminal cannot
// show, so content is never lost without saying so.
const elisionNotice = "…"

// clip bounds a modal's content to the room inside its border and padding. A
// modal that sizes itself is already within bounds and passes through
// untouched; this is the guarantee for the ones that do not.
func clip(content string, width, height int) string {
	if width < 1 || height < 1 {
		return content
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = table.Truncate(line, width)
	}

	if len(lines) > height {
		lines = append(lines[:height-1:height-1], ui.HelpStyle.Render(elisionNotice))
	}

	return strings.Join(lines, "\n")
}

// ClosedMsg is sent when a modal is closed.
type ClosedMsg struct {
	Result any
}
