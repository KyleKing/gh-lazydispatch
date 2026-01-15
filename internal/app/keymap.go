package app

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keyboard shortcuts for the application.
type KeyMap struct {
	Branch   key.Binding
	Copy     key.Binding
	Down     key.Binding
	Edit     key.Binding
	Enter    key.Binding
	Escape   key.Binding
	Filter   key.Binding
	Help     key.Binding
	Quit     key.Binding
	Reset    key.Binding
	ShiftTab key.Binding
	Tab      key.Binding
	Up       key.Binding
	Watch    key.Binding

	Input0 key.Binding
	Input1 key.Binding
	Input2 key.Binding
	Input3 key.Binding
	Input4 key.Binding
	Input5 key.Binding
	Input6 key.Binding
	Input7 key.Binding
	Input8 key.Binding
	Input9 key.Binding

	Workflow0 key.Binding
	Workflow1 key.Binding
	Workflow2 key.Binding
	Workflow3 key.Binding
	Workflow4 key.Binding
	Workflow5 key.Binding
	Workflow6 key.Binding
	Workflow7 key.Binding
	Workflow8 key.Binding
	Workflow9 key.Binding
}

// DefaultKeyMap returns the default keyboard shortcuts.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Branch:   key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "branch")),
		Copy:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy to clipboard")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/run")),
		Escape:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Reset:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reset inputs")),
		ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev pane")),
		Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane")),
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Watch:    key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "watch")),

		Input0: key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "input 0")),
		Input1: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "input 1")),
		Input2: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "input 2")),
		Input3: key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "input 3")),
		Input4: key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "input 4")),
		Input5: key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "input 5")),
		Input6: key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "input 6")),
		Input7: key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "input 7")),
		Input8: key.NewBinding(key.WithKeys("8"), key.WithHelp("8", "input 8")),
		Input9: key.NewBinding(key.WithKeys("9"), key.WithHelp("9", "input 9")),

		Workflow0: key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "workflow all")),
		Workflow1: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "workflow 1")),
		Workflow2: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "workflow 2")),
		Workflow3: key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "workflow 3")),
		Workflow4: key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "workflow 4")),
		Workflow5: key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "workflow 5")),
		Workflow6: key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "workflow 6")),
		Workflow7: key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "workflow 7")),
		Workflow8: key.NewBinding(key.WithKeys("8"), key.WithHelp("8", "workflow 8")),
		Workflow9: key.NewBinding(key.WithKeys("9"), key.WithHelp("9", "workflow 9")),
	}
}

// ShortHelp returns a short list of key bindings for the help view.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Enter, k.Branch, k.Filter, k.Quit, k.Help}
}

// FullHelp returns the full list of key bindings for the help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab, k.ShiftTab, k.Up, k.Down},
		{k.Enter, k.Edit, k.Escape, k.Branch},
		{k.Watch, k.Filter, k.Copy, k.Reset},
		{k.Input1, k.Input2, k.Input3, k.Input0},
		{k.Quit, k.Help},
	}
}
