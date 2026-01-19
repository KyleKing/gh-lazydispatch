package testutil

import (
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// WorkflowFixture creates a test workflow with specified properties.
func WorkflowFixture(name, filename string, inputs map[string]workflow.WorkflowInput) workflow.WorkflowFile {
	return workflow.WorkflowFile{
		Name:     name,
		Filename: filename,
		On: workflow.OnTrigger{
			WorkflowDispatch: &workflow.WorkflowDispatch{
				Inputs: inputs,
			},
		},
	}
}

// SimpleWorkflow creates a basic workflow with no inputs.
func SimpleWorkflow(name, filename string) workflow.WorkflowFile {
	return WorkflowFixture(name, filename, nil)
}

// WorkflowWithChoice creates a workflow with a choice input.
func WorkflowWithChoice(name, filename, inputName string, options []string, defaultVal string) workflow.WorkflowFile {
	return WorkflowFixture(name, filename, map[string]workflow.WorkflowInput{
		inputName: {
			Type:    "choice",
			Default: defaultVal,
			Options: options,
		},
	})
}

// WorkflowWithBoolean creates a workflow with a boolean input.
func WorkflowWithBoolean(name, filename, inputName string, defaultVal bool) workflow.WorkflowFile {
	defaultStr := "false"
	if defaultVal {
		defaultStr = "true"
	}
	return WorkflowFixture(name, filename, map[string]workflow.WorkflowInput{
		inputName: {
			Type:    "boolean",
			Default: defaultStr,
		},
	})
}

// WorkflowWithString creates a workflow with a string input.
func WorkflowWithString(name, filename, inputName, defaultVal, description string) workflow.WorkflowFile {
	return WorkflowFixture(name, filename, map[string]workflow.WorkflowInput{
		inputName: {
			Type:        "string",
			Default:     defaultVal,
			Description: description,
		},
	})
}

// NewTestHistory creates a history store with test data.
func NewTestHistory(repo string, records []struct {
	Workflow string
	Branch   string
	Inputs   map[string]string
}) *frecency.Store {
	store := frecency.NewStore()
	for _, record := range records {
		store.Record(repo, record.Workflow, record.Branch, record.Inputs)
	}
	return store
}

// AssertEqual fails the test if got != want.
func AssertEqual[T comparable](t *testing.T, got, want T, msgAndArgs ...interface{}) {
	t.Helper()
	if got != want {
		if len(msgAndArgs) > 0 {
			format := msgAndArgs[0].(string)
			args := msgAndArgs[1:]
			t.Errorf(format+": got %v, want %v", append(args, got, want)...)
		} else {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

// AssertNotEqual fails the test if got == want.
func AssertNotEqual[T comparable](t *testing.T, got, want T, msgAndArgs ...interface{}) {
	t.Helper()
	if got == want {
		if len(msgAndArgs) > 0 {
			format := msgAndArgs[0].(string)
			args := msgAndArgs[1:]
			t.Errorf(format+": got %v, want != %v", append(args, got, want)...)
		} else {
			t.Errorf("got %v, want != %v", got, want)
		}
	}
}

// AssertNil fails the test if value is not nil.
func AssertNil(t *testing.T, value interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if value != nil {
		if len(msgAndArgs) > 0 {
			format := msgAndArgs[0].(string)
			args := msgAndArgs[1:]
			t.Errorf(format+": expected nil, got %v", append(args, value)...)
		} else {
			t.Errorf("expected nil, got %v", value)
		}
	}
}

// AssertNotNil fails the test if value is nil.
func AssertNotNil(t *testing.T, value interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if value == nil {
		if len(msgAndArgs) > 0 {
			format := msgAndArgs[0].(string)
			args := msgAndArgs[1:]
			t.Errorf(format+": expected non-nil value", args...)
		} else {
			t.Error("expected non-nil value")
		}
	}
}

// AssertTrue fails the test if condition is false.
func AssertTrue(t *testing.T, condition bool, msgAndArgs ...interface{}) {
	t.Helper()
	if !condition {
		if len(msgAndArgs) > 0 {
			format := msgAndArgs[0].(string)
			args := msgAndArgs[1:]
			t.Errorf(format, args...)
		} else {
			t.Error("expected true, got false")
		}
	}
}

// AssertFalse fails the test if condition is true.
func AssertFalse(t *testing.T, condition bool, msgAndArgs ...interface{}) {
	t.Helper()
	if condition {
		if len(msgAndArgs) > 0 {
			format := msgAndArgs[0].(string)
			args := msgAndArgs[1:]
			t.Errorf(format, args...)
		} else {
			t.Error("expected false, got true")
		}
	}
}

// AssertContains fails the test if haystack doesn't contain needle.
func AssertContains(t *testing.T, haystack, needle string, msgAndArgs ...interface{}) {
	t.Helper()
	if !contains(haystack, needle) {
		if len(msgAndArgs) > 0 {
			format := msgAndArgs[0].(string)
			args := msgAndArgs[1:]
			t.Errorf(format+": %q not found in %q", append(args, needle, haystack)...)
		} else {
			t.Errorf("%q not found in %q", needle, haystack)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
