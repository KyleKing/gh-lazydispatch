package validation_test

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/validation"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// A dispatch that GitHub will reject is worth rejecting locally, where the
// message can name the input rather than arriving as a 422 after the round
// trip.
func TestValidateDispatch_NamesWhatWouldBeRejected(t *testing.T) {
	t.Parallel()

	inputs := map[string]workflow.Input{
		"env":     {Type: "choice", Options: []string{"production", "staging"}},
		"dry_run": {Type: "boolean"},
		"tag":     {Required: true},
		"note":    {},
	}

	tests := []struct {
		name    string
		values  map[string]string
		wantFor string
		want    string
	}{
		{
			name:   "everything set",
			values: map[string]string{"env": "staging", "dry_run": "false", "tag": "v1", "note": ""},
		},
		{
			name:    "a choice outside its options",
			values:  map[string]string{"env": "qa", "dry_run": "true", "tag": "v1"},
			wantFor: "env", want: "not one of production, staging",
		},
		{
			name:    "a boolean that is neither",
			values:  map[string]string{"env": "staging", "dry_run": "yes", "tag": "v1"},
			wantFor: "dry_run", want: "neither true nor false",
		},
		{
			name:    "a required input left blank",
			values:  map[string]string{"env": "staging", "dry_run": "true"},
			wantFor: "tag", want: "is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := validation.ValidateDispatch(inputs, tt.values)

			if tt.wantFor == "" {
				if errs != nil {
					t.Fatalf("a valid set reported %v", errs)
				}

				return
			}

			messages, ok := errs[tt.wantFor]
			if !ok {
				t.Fatalf("nothing was reported for %s, got %v", tt.wantFor, errs)
			}

			if !strings.Contains(strings.Join(messages, " "), tt.want) {
				t.Errorf("%s reported %v, want it to mention %q", tt.wantFor, messages, tt.want)
			}

			// Only the offending input is named, so the modal does not list
			// every input a workflow declares.
			if len(errs) != 1 {
				t.Errorf("one bad value reported %d inputs: %v", len(errs), errs)
			}
		})
	}
}

// An empty environment list means the names could not be read, which is not
// the same as the value being wrong.
func TestValidateEnvironment_StaysQuietWhereTheNamesAreUnknown(t *testing.T) {
	t.Parallel()

	known := []string{"production", "staging"}

	if got := validation.ValidateEnvironment("qa", known); got == "" {
		t.Error("an environment the repository does not have was accepted")
	}

	if got := validation.ValidateEnvironment("staging", known); got != "" {
		t.Errorf("a real environment was rejected: %s", got)
	}

	if got := validation.ValidateEnvironment("qa", nil); got != "" {
		t.Errorf("a value was rejected against a list that could not be read: %s", got)
	}
}
