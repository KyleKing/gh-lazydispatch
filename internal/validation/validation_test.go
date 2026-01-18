package validation

import (
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

func TestValidateHistoryConfig(t *testing.T) {
	tests := []struct {
		name       string
		entry      *frecency.HistoryEntry
		wf         *workflow.WorkflowFile
		wantErrors int
		checkError func(t *testing.T, errs []ConfigValidationError)
	}{
		{
			name:       "nil entry",
			entry:      nil,
			wf:         &workflow.WorkflowFile{},
			wantErrors: 0,
		},
		{
			name:       "nil workflow",
			entry:      &frecency.HistoryEntry{},
			wf:         nil,
			wantErrors: 0,
		},
		{
			name: "valid config",
			entry: &frecency.HistoryEntry{
				Inputs: map[string]string{
					"environment": "production",
				},
			},
			wf: &workflow.WorkflowFile{
				On: workflow.OnTrigger{
					WorkflowDispatch: &workflow.WorkflowDispatch{
						Inputs: map[string]workflow.WorkflowInput{
							"environment": {Type: "string"},
						},
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "missing input name",
			entry: &frecency.HistoryEntry{
				Inputs: map[string]string{
					"old_env": "production",
				},
			},
			wf: &workflow.WorkflowFile{
				On: workflow.OnTrigger{
					WorkflowDispatch: &workflow.WorkflowDispatch{
						Inputs: map[string]workflow.WorkflowInput{
							"environment": {Type: "string"},
						},
					},
				},
			},
			wantErrors: 1,
			checkError: func(t *testing.T, errs []ConfigValidationError) {
				if errs[0].Status != StatusMissing {
					t.Errorf("expected StatusMissing, got %v", errs[0].Status)
				}
				if errs[0].HistoricalName != "old_env" {
					t.Errorf("expected historical name 'old_env', got %q", errs[0].HistoricalName)
				}
			},
		},
		{
			name: "choice value not in options",
			entry: &frecency.HistoryEntry{
				Inputs: map[string]string{
					"environment": "development",
				},
			},
			wf: &workflow.WorkflowFile{
				On: workflow.OnTrigger{
					WorkflowDispatch: &workflow.WorkflowDispatch{
						Inputs: map[string]workflow.WorkflowInput{
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
				Inputs: map[string]string{
					"environment": "production",
				},
			},
			wf: &workflow.WorkflowFile{
				On: workflow.OnTrigger{
					WorkflowDispatch: &workflow.WorkflowDispatch{
						Inputs: map[string]workflow.WorkflowInput{
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
		{
			name: "multiple errors",
			entry: &frecency.HistoryEntry{
				Inputs: map[string]string{
					"old_env":    "production",
					"old_region": "us-east-1",
					"version":    "v2",
				},
			},
			wf: &workflow.WorkflowFile{
				On: workflow.OnTrigger{
					WorkflowDispatch: &workflow.WorkflowDispatch{
						Inputs: map[string]workflow.WorkflowInput{
							"environment": {Type: "string"},
							"region":      {Type: "string"},
							"version": {
								Type:    "choice",
								Options: []string{"v1", "v3"},
							},
						},
					},
				},
			},
			wantErrors: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestFindBestMatch(t *testing.T) {
	tests := []struct {
		name           string
		historicalName string
		currentInputs  map[string]workflow.WorkflowInput
		wantSuggestion string
	}{
		{
			name:           "empty inputs",
			historicalName: "old_env",
			currentInputs:  map[string]workflow.WorkflowInput{},
			wantSuggestion: "",
		},
		{
			name:           "exact match candidate",
			historicalName: "environment",
			currentInputs: map[string]workflow.WorkflowInput{
				"environment": {Type: "string"},
			},
			wantSuggestion: "environment",
		},
		{
			name:           "similar name",
			historicalName: "env",
			currentInputs: map[string]workflow.WorkflowInput{
				"environment": {Type: "string"},
				"region":      {Type: "string"},
			},
			wantSuggestion: "environment",
		},
		{
			name:           "returns best fuzzy match",
			historicalName: "envirn",
			currentInputs: map[string]workflow.WorkflowInput{
				"environment": {Type: "string"},
				"debug":       {Type: "boolean"},
			},
			wantSuggestion: "environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findBestMatch(tt.historicalName, tt.currentInputs)

			if got != tt.wantSuggestion {
				t.Errorf("findBestMatch() = %q, want %q", got, tt.wantSuggestion)
			}
		})
	}
}

func TestValidateInputValue(t *testing.T) {
	tests := []struct {
		name      string
		inputName string
		value     string
		input     workflow.WorkflowInput
		wantError bool
		wantStatus ValidationStatus
	}{
		{
			name:      "string type valid",
			inputName: "message",
			value:     "hello world",
			input:     workflow.WorkflowInput{Type: "string"},
			wantError: false,
		},
		{
			name:      "choice valid option",
			inputName: "environment",
			value:     "production",
			input: workflow.WorkflowInput{
				Type:    "choice",
				Options: []string{"production", "staging"},
			},
			wantError: false,
		},
		{
			name:      "choice invalid option",
			inputName: "environment",
			value:     "development",
			input: workflow.WorkflowInput{
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
			input: workflow.WorkflowInput{
				Type:    "choice",
				Options: []string{},
			},
			wantError: false,
		},
		{
			name:      "boolean type",
			inputName: "debug",
			value:     "true",
			input:     workflow.WorkflowInput{Type: "boolean"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
