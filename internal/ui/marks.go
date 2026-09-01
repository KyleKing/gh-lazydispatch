package ui

import (
	"maps"
	"slices"
)

// MarkSet is the set of rows a verb acts on instead of the cursor: `space`
// marks a row, and the action menu then offers the verb over the marked set.
// Keys are whatever the pane names its rows by, so a set outlives a reordering
// of the list it came from.
type MarkSet struct {
	keys map[string]struct{}
}

// Toggle adds or removes one row.
func (s *MarkSet) Toggle(key string) {
	if s.keys == nil {
		s.keys = make(map[string]struct{})
	}

	if _, ok := s.keys[key]; ok {
		delete(s.keys, key)

		return
	}

	s.keys[key] = struct{}{}
}

// Has reports whether a row is marked.
func (s MarkSet) Has(key string) bool {
	_, ok := s.keys[key]

	return ok
}

// Len is how many rows are marked.
func (s MarkSet) Len() int { return len(s.keys) }

// Keys are the marked rows, sorted so a batch runs in a stable order.
func (s MarkSet) Keys() []string {
	return slices.Sorted(maps.Keys(s.keys))
}

// Clear drops every mark, which is what finishing a batch verb does: leaving
// them set would make the next verb act on rows the reader has moved past.
func (s *MarkSet) Clear() { s.keys = nil }

// MarkGlyph is the cell a list spends on whether a row is marked.
func MarkGlyph(marked bool) string {
	if marked {
		return "*"
	}

	return " "
}
