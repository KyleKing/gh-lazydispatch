package modal

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/overlay"

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

	return Overlay(background, s.Current().View(), s.width, s.height)
}

// Overlay centers content over background in the same frame a modal gets. It
// is exported for a view that is normally a pane and is promoted to an overlay
// where its pane is too short to hold it, which needs the modal's look without
// the stack's key routing.
func Overlay(_, content string, width, height int) string {
	return overlay.Center(ground(content, ui.ModalBgColor), width, height, overlayStyles())
}

// OverlayWidth is the room an overlay's content has inside its border and
// padding, which is what a caller wraps prose to before handing it over.
func OverlayWidth(width int) int { return overlay.ContentWidth(width, overlayStyles()) }

// A modal is read, not lived in, so it spends one row above and below its
// content rather than framing a short list in blank space.
const (
	modalPaddingVertical   = 1
	modalPaddingHorizontal = 3
)

// The border and padding a modal's content sits inside of, which
// overlay.ContentWidth and ContentHeight report for the same frame.
const (
	modalChromeVertical   = 2 + 2*modalPaddingVertical
	modalChromeHorizontal = 2 + 2*modalPaddingHorizontal
)

// overlayStyles are the faces a modal's frame draws with.
func overlayStyles() overlay.Styles {
	return overlay.Styles{
		Frame: lipgloss.NewStyle().
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(ui.PrimaryColor).
			Padding(modalPaddingVertical, modalPaddingHorizontal).
			Background(ui.ModalBgColor),
		Elision: ui.HelpStyle,
	}
}

// ClosedMsg is sent when a modal is closed.
type ClosedMsg struct {
	Result any
}
