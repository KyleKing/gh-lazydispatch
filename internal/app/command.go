package app

import "github.com/kyleking/aragonite/tui/commandbar"

// Command is one named `:command` the command bar can run. A command exists
// so an action has a name a reader can guess, which is what a key cannot
// offer once the letters run out.
type Command = commandbar.Command[Model]

// Candidate is one completion paired with the line explaining it, so the
// command bar shows both without a second lookup.
type Candidate = commandbar.Candidate

// Registry holds the commands the bar knows, sorted by name.
type Registry = commandbar.Registry[Model]

// NewRegistry builds a Registry, sorting by name so every listing and
// completion comes back in one order.
var NewRegistry = commandbar.NewRegistry[Model]
