package app

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keyboard shortcuts for the application.
type KeyMap struct {
	Branch   key.Binding
	Chain    key.Binding
	Copy     key.Binding
	Down     key.Binding
	Edit     key.Binding
	Enter    key.Binding
	Escape   key.Binding
	Filter   key.Binding
	Help     key.Binding
	LiveView key.Binding
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

// makeNumberedBinding creates a key binding for a numbered shortcut.
func makeNumberedBinding(num int, prefix string) key.Binding {
	numStr := string('0' + rune(num))
	label := numStr
	if prefix == "workflow" && num == 0 {
		return key.NewBinding(key.WithKeys(numStr), key.WithHelp(label, "workflow all"))
	}
	return key.NewBinding(key.WithKeys(numStr), key.WithHelp(label, prefix+" "+numStr))
}

// DefaultKeyMap returns the default keyboard shortcuts.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Branch:   key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "branch")),
		Chain:    key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "run chain")),
		Copy:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy to clipboard")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/run")),
		Escape:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		LiveView: key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "live view")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Reset:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reset inputs")),
		ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev pane")),
		Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane")),
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Watch:    key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "watch")),

		Input0: makeNumberedBinding(0, "input"),
		Input1: makeNumberedBinding(1, "input"),
		Input2: makeNumberedBinding(2, "input"),
		Input3: makeNumberedBinding(3, "input"),
		Input4: makeNumberedBinding(4, "input"),
		Input5: makeNumberedBinding(5, "input"),
		Input6: makeNumberedBinding(6, "input"),
		Input7: makeNumberedBinding(7, "input"),
		Input8: makeNumberedBinding(8, "input"),
		Input9: makeNumberedBinding(9, "input"),

		Workflow0: makeNumberedBinding(0, "workflow"),
		Workflow1: makeNumberedBinding(1, "workflow"),
		Workflow2: makeNumberedBinding(2, "workflow"),
		Workflow3: makeNumberedBinding(3, "workflow"),
		Workflow4: makeNumberedBinding(4, "workflow"),
		Workflow5: makeNumberedBinding(5, "workflow"),
		Workflow6: makeNumberedBinding(6, "workflow"),
		Workflow7: makeNumberedBinding(7, "workflow"),
		Workflow8: makeNumberedBinding(8, "workflow"),
		Workflow9: makeNumberedBinding(9, "workflow"),
	}
}

// InputKeys returns all input key bindings as a slice indexed 0-9.
func (k KeyMap) InputKeys() []key.Binding {
	return []key.Binding{
		k.Input0, k.Input1, k.Input2, k.Input3, k.Input4,
		k.Input5, k.Input6, k.Input7, k.Input8, k.Input9,
	}
}

// WorkflowKeys returns all workflow key bindings as a slice indexed 0-9.
func (k KeyMap) WorkflowKeys() []key.Binding {
	return []key.Binding{
		k.Workflow0, k.Workflow1, k.Workflow2, k.Workflow3, k.Workflow4,
		k.Workflow5, k.Workflow6, k.Workflow7, k.Workflow8, k.Workflow9,
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
