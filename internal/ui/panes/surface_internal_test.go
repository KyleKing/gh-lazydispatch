package panes

import (
	"reflect"
	"testing"
)

// The panel draws its own border and calls each child's ViewContent, so a
// child that also has a View renders into nothing. A test asserting on that
// string passes while the user looks at something else.
func TestPanes_NothingInsideThePanelHasAnUnrenderedView(t *testing.T) {
	t.Parallel()

	children := []any{RunsModel{}, LiveRunsModel{}, HistoryModel{}, FlakyModel{}, TimelineModel{}}

	for _, child := range children {
		typ := reflect.TypeOf(child)

		if _, ok := typ.MethodByName("ViewContent"); !ok {
			t.Errorf("%s is drawn by the panel and has no ViewContent", typ.Name())
		}

		if _, ok := typ.MethodByName("View"); ok {
			t.Errorf("%s has a View the panel never calls, so its output reaches no one", typ.Name())
		}
	}
}
