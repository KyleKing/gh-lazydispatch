package app

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Command is one named `:command` the command bar can run. A command exists
// so an action has a name a reader can guess, which is what a key cannot
// offer once the letters run out.
type Command struct {
	// Run applies the command. Args are the whitespace-separated words after
	// the command's own name.
	Run func(Model, []string) (Model, tea.Cmd)
	// Complete offers the candidates for the argument being typed, which is
	// the last element of args and may be empty. A nil Complete means the
	// command takes no arguments worth completing.
	Complete    func(Model, []string) []Candidate
	Name        string
	Description string
}

// Candidate is one completion paired with the line explaining it, so the
// command bar shows both without a second lookup.
type Candidate struct {
	Name        string
	Description string
}

// Registry holds the commands the bar knows, sorted by name.
type Registry struct {
	commands []Command
}

// NewRegistry builds a Registry, sorting by name so every listing and
// completion comes back in one order.
func NewRegistry(commands ...Command) Registry {
	sorted := make([]Command, len(commands))
	copy(sorted, commands)

	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	return Registry{commands: sorted}
}

// Commands returns every registered command.
func (r Registry) Commands() []Command {
	return r.commands
}

// Lookup resolves a name exactly, or by a prefix that only one command
// answers to. An ambiguous prefix resolves to nothing rather than to the
// first match, so `:c` never silently runs whichever command sorts first.
func (r Registry) Lookup(name string) (Command, bool) {
	var matches []Command

	for _, command := range r.commands {
		if command.Name == name {
			return command, true
		}

		if strings.HasPrefix(command.Name, name) {
			matches = append(matches, command)
		}
	}

	if len(matches) == 1 {
		return matches[0], true
	}

	return Command{}, false
}

// Candidates returns the commands whose names start with prefix.
func (r Registry) Candidates(prefix string) []Candidate {
	candidates := make([]Candidate, 0, len(r.commands))

	for _, command := range r.commands {
		if strings.HasPrefix(command.Name, prefix) {
			candidates = append(candidates, Candidate{Name: command.Name, Description: command.Description})
		}
	}

	return candidates
}

// Listing renders every command as "name<tab>description", one per line, for
// a caller with no help modal to open.
func (r Registry) Listing() string {
	var b strings.Builder

	for _, command := range r.commands {
		b.WriteString(command.Name)
		b.WriteString("\t")
		b.WriteString(command.Description)
		b.WriteString("\n")
	}

	return strings.TrimSuffix(b.String(), "\n")
}

// completionsFor returns the candidates for the word being typed in line, and
// reports whether anything can complete it. Completing the first word offers
// commands; completing a later one asks that command.
func (r Registry) completionsFor(m Model, line string) ([]Candidate, bool) {
	fields := strings.Fields(line)
	typingNewWord := line == "" || strings.HasSuffix(line, " ")

	if len(fields) == 0 || (len(fields) == 1 && !typingNewWord) {
		prefix := ""
		if len(fields) == 1 {
			prefix = fields[0]
		}

		return r.Candidates(prefix), true
	}

	command, found := r.Lookup(fields[0])
	if !found || command.Complete == nil {
		return nil, false
	}

	args := fields[1:]
	if typingNewWord {
		args = append(args, "")
	}

	return command.Complete(m, args), true
}

// commonPrefix returns the longest prefix every candidate shares, which is how
// far one tab press can fill in without choosing for the user.
func commonPrefix(candidates []Candidate) string {
	if len(candidates) == 0 {
		return ""
	}

	prefix := candidates[0].Name
	for _, candidate := range candidates[1:] {
		for !strings.HasPrefix(candidate.Name, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}

	return prefix
}
