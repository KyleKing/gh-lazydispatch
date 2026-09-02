package validation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

const (
	inputTypeBoolean = "boolean"
	boolTrueValue    = "true"
	boolFalseValue   = "false"
)

// ValidateDispatch names what is wrong with a set of input values before the
// dispatch goes out, keyed by input name. A choice outside its options and a
// boolean that is not true or false are what GitHub rejects; a required input
// left blank is this tool's own rule, since a blank one is a mistake far more
// often than it is a value.
func ValidateDispatch(inputs map[string]workflow.Input, values map[string]string) map[string][]string {
	errs := make(map[string][]string)

	for name, input := range inputs {
		value := values[name]

		if message := dispatchError(input, value); message != "" {
			errs[name] = append(errs[name], message)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func dispatchError(input workflow.Input, value string) string {
	if input.Required && value == "" {
		return "is required and has no value"
	}

	switch input.InputType() {
	case inputTypeChoice:
		if len(input.Options) > 0 && !slices.Contains(input.Options, value) {
			return fmt.Sprintf("is %q, which is not one of %s", value, strings.Join(input.Options, ", "))
		}
	case inputTypeBoolean:
		if value != boolTrueValue && value != boolFalseValue {
			return fmt.Sprintf("is %q, which is neither true nor false", value)
		}
	}

	return ""
}

// ValidateEnvironment names an environment input set to something the
// repository does not have. It is separate because the environments are read
// from the API rather than declared by the workflow, and an empty list means
// they could not be read rather than that none exist.
func ValidateEnvironment(value string, environments []string) string {
	if value == "" || len(environments) == 0 || slices.Contains(environments, value) {
		return ""
	}

	return fmt.Sprintf("is %q, which the repository has no environment named", value)
}
