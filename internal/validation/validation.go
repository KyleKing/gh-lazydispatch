// Package validation provides input validation and history-based suggestions for workflow inputs.
package validation

import (
	"sort"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
	"github.com/sahilm/fuzzy"
)

// ValidationStatus represents the validation state of a historical input.
type ValidationStatus int

const (
	StatusValid          ValidationStatus = iota // Input is valid
	StatusMissing                                // Input name no longer exists
	StatusTypeChanged                            // Input type has changed
	StatusOptionsChanged                         // Value not in choice options
)

// ConfigValidationError represents a validation error for a historical input.
type ConfigValidationError struct {
	HistoricalName  string
	HistoricalValue string
	Status          ValidationStatus
	Suggestion      string // Suggested input name for remapping
}

// ValidateHistoryConfig validates a historical configuration against current workflow inputs.
// Returns a list of validation errors, or nil if all inputs are valid.
func ValidateHistoryConfig(entry *frecency.HistoryEntry, wf *workflow.WorkflowFile) []ConfigValidationError {
	if entry == nil || wf == nil {
		return nil
	}

	var errors []ConfigValidationError

	currentInputs := wf.GetInputs()

	for historicalName, historicalValue := range entry.Inputs {
		currentInput, exists := currentInputs[historicalName]

		if !exists {
			// Input name no longer exists - try to find a suggestion
			suggestion := findBestMatch(historicalName, currentInputs)
			errors = append(errors, ConfigValidationError{
				HistoricalName:  historicalName,
				HistoricalValue: historicalValue,
				Status:          StatusMissing,
				Suggestion:      suggestion,
			})

			continue
		}

		// Input exists - validate value compatibility
		if err := validateInputValue(historicalName, historicalValue, currentInput); err != nil {
			errors = append(errors, *err)
		}
	}

	return errors
}

// validateInputValue checks if a historical value is compatible with the current input definition.
func validateInputValue(name, value string, input workflow.WorkflowInput) *ConfigValidationError {
	// For choice inputs, validate that the value is still in the options
	if input.Type == "choice" && len(input.Options) > 0 {
		validOption := false

		for _, option := range input.Options {
			if value == option {
				validOption = true
				break
			}
		}

		if !validOption {
			return &ConfigValidationError{
				HistoricalName:  name,
				HistoricalValue: value,
				Status:          StatusOptionsChanged,
				Suggestion:      input.Default, // Suggest the current default
			}
		}
	}

	// Could add more type-specific validations here (boolean, number, etc.)

	return nil
}

// findBestMatch uses fuzzy matching to find the most similar input name.
// Returns empty string if no good match is found.
func findBestMatch(historicalName string, currentInputs map[string]workflow.WorkflowInput) string {
	if len(currentInputs) == 0 {
		return ""
	}

	// Build list of current input names
	names := make([]string, 0, len(currentInputs))
	for name := range currentInputs {
		names = append(names, name)
	}

	sort.Strings(names)

	// Use fuzzy matching to find similar names
	matches := fuzzy.Find(historicalName, names)
	if len(matches) == 0 {
		return ""
	}

	// Return the best match (highest score)
	bestMatch := matches[0]

	// Only suggest if the match score is reasonable (avoid poor suggestions)
	// The fuzzy library scores matches, with lower being better match
	// We'll suggest the top match if it exists
	if bestMatch.Score >= 0 {
		return bestMatch.Str
	}

	return ""
}
