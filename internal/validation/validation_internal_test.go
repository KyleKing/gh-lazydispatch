package validation

import (
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// validateHistoryConfigCase is a single ValidateHistoryConfig table-test case,
// shared across the TestValidateHistoryConfig_* functions.
type validateHistoryConfigCase struct {
	name       string
	entry      *frecency.HistoryEntry
	wf         *workflow.File
	wantErrors int
	checkError func(t *testing.T, errs []ConfigValidationError)
}

// runValidateHistoryConfigCases runs ValidateHistoryConfig against each case
// and checks the error count plus any case-specific assertions.
func runValidateHistoryConfigCases(t *testing.T, tests []validateHistoryConfigCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := ValidateHistoryConfig(tt.entry, tt.wf)

			if len(errs) != tt.wantErrors {
				t.Errorf("ValidateHistoryConfig() errors = %d, want %d", len(errs), tt.wantErrors)
			}

			if tt.checkError != nil && len(errs) > 0 {
				tt.checkError(t, errs)
			}
		})
	}
}

func TestValidateHistoryConfig_NilInputsAndValid(t *testing.T) {
	t.Parallel()

	runValidateHistoryConfigCases(t, []validateHistoryConfigCase{
		{name: "nil entry", entry: nil, wf: &workflow.File{}, wantErrors: 0},
		{name: "nil workflow", entry: &frecency.HistoryEntry{}, wf: nil, wantErrors: 0},
		{
			name: "valid config",
			entry: &frecency.HistoryEntry{
				Inputs: map[string]string{"environment": "production"},
			},
			wf: &workflow.File{
				On: workflow.OnTrigger{
					Dispatch: &workflow.Dispatch{
						Inputs: map[string]workflow.Input{"environment": {Type: "string"}},
					},
				},
			},
			wantErrors: 0,
		},
	})
}

func TestValidateHistoryConfig_MissingInput(t *testing.T) {
	t.Parallel()

	runValidateHistoryConfigCases(t, []validateHistoryConfigCase{
		{
			name: "missing input name",
			entry: &frecency.HistoryEntry{
				Inputs: map[string]string{"old_env": "production"},
			},
			wf: &workflow.File{
				On: workflow.OnTrigger{
					Dispatch: &workflow.Dispatch{
						Inputs: map[string]workflow.Input{"environment": {Type: "string"}},
					},
				},
			},
			wantErrors: 1,
			checkError: func(t *testing.T, errs []ConfigValidationError) {
				t.Helper()

				if errs[0].Status != StatusMissing {
					t.Errorf("expected StatusMissing, got %v", errs[0].Status)
				}
				if errs[0].HistoricalName != "old_env" {
					t.Errorf("expected historical name 'old_env', got %q", errs[0].HistoricalName)
				}
			},
		},
	})
}

func TestValidateHistoryConfig_ChoiceOptions(t *testing.T) {
	t.Parallel()

	runValidateHistoryConfigCases(t, []validateHistoryConfigCase{
		{
			name: "choice value not in options",
			entry: &frecency.HistoryEntry{
				Inputs: map[string]string{"environment": "development"},
			},
			wf: &workflow.File{
				On: workflow.OnTrigger{
					Dispatch: &workflow.Dispatch{
						Inputs: map[string]workflow.Input{
							"environment": {
								Type:    "choice",
								Options: []string{"production", "staging"},
								Default: "staging",
							},
						},
					},
				},
			},
			wantErrors: 1,
			checkError: func(t *testing.T, errs []ConfigValidationError) {
				t.Helper()

				if errs[0].Status != StatusOptionsChanged {
					t.Errorf("expected StatusOptionsChanged, got %v", errs[0].Status)
				}
				if errs[0].HistoricalValue != "development" {
					t.Errorf("expected historical value 'development', got %q", errs[0].HistoricalValue)
				}
				if errs[0].Suggestion != "staging" {
					t.Errorf("expected suggestion 'staging', got %q", errs[0].Suggestion)
				}
			},
		},
		{
			name: "choice value in options",
			entry: &frecency.HistoryEntry{
				Inputs: map[string]string{"environment": "production"},
			},
			wf: &workflow.File{
				On: workflow.OnTrigger{
					Dispatch: &workflow.Dispatch{
						Inputs: map[string]workflow.Input{
							"environment": {
								Type:    "choice",
								Options: []string{"production", "staging"},
							},
						},
					},
				},
			},
			wantErrors: 0,
		},
	})
}

func TestValidateHistoryConfig_MultipleErrors(t *testing.T) {
	t.Parallel()

	runValidateHistoryConfigCases(t, []validateHistoryConfigCase{
		{
			name: "multiple errors",
			entry: &frecency.HistoryEntry{
				Inputs: map[string]string{
					"old_env":    "production",
					"old_region": "us-east-1",
					"version":    "v2",
				},
			},
			wf: &workflow.File{
				On: workflow.OnTrigger{
					Dispatch: &workflow.Dispatch{
						Inputs: map[string]workflow.Input{
							"environment": {Type: "string"},
							"region":      {Type: "string"},
							"version":     {Type: "choice", Options: []string{"v1", "v3"}},
						},
					},
				},
			},
			wantErrors: 3,
		},
	})
}

func TestFindBestMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		historicalName string
		currentInputs  map[string]workflow.Input
		wantSuggestion string
	}{
		{
			name:           "empty inputs",
			historicalName: "old_env",
			currentInputs:  map[string]workflow.Input{},
			wantSuggestion: "",
		},
		{
			name:           "exact match candidate",
			historicalName: "environment",
			currentInputs: map[string]workflow.Input{
				"environment": {Type: "string"},
			},
			wantSuggestion: "environment",
		},
		{
			name:           "similar name",
			historicalName: "env",
			currentInputs: map[string]workflow.Input{
				"environment": {Type: "string"},
				"region":      {Type: "string"},
			},
			wantSuggestion: "environment",
		},
		{
			name:           "returns best fuzzy match",
			historicalName: "envirn",
			currentInputs: map[string]workflow.Input{
				"environment": {Type: "string"},
				"debug":       {Type: "boolean"},
			},
			wantSuggestion: "environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := findBestMatch(tt.historicalName, tt.currentInputs)

			if got != tt.wantSuggestion {
				t.Errorf("findBestMatch() = %q, want %q", got, tt.wantSuggestion)
			}
		})
	}
}

func TestValidateInputValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputName  string
		value      string
		input      workflow.Input
		wantError  bool
		wantStatus Status
	}{
		{
			name:      "string type valid",
			inputName: "message",
			value:     "hello world",
			input:     workflow.Input{Type: "string"},
			wantError: false,
		},
		{
			name:      "choice valid option",
			inputName: "environment",
			value:     "production",
			input: workflow.Input{
				Type:    "choice",
				Options: []string{"production", "staging"},
			},
			wantError: false,
		},
		{
			name:      "choice invalid option",
			inputName: "environment",
			value:     "development",
			input: workflow.Input{
				Type:    "choice",
				Options: []string{"production", "staging"},
				Default: "staging",
			},
			wantError:  true,
			wantStatus: StatusOptionsChanged,
		},
		{
			name:      "choice empty options",
			inputName: "environment",
			value:     "production",
			input: workflow.Input{
				Type:    "choice",
				Options: []string{},
			},
			wantError: false,
		},
		{
			name:      "boolean type",
			inputName: "debug",
			value:     "true",
			input:     workflow.Input{Type: "boolean"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateInputValue(tt.inputName, tt.value, tt.input)

			if (err != nil) != tt.wantError {
				t.Errorf("validateInputValue() error = %v, wantError %v", err, tt.wantError)
			}

			if err != nil && err.Status != tt.wantStatus {
				t.Errorf("validateInputValue() status = %v, want %v", err.Status, tt.wantStatus)
			}
		})
	}
}
