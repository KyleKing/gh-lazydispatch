package chain

import (
	"regexp"
	"strings"
)

// InterpolationContext provides values for template interpolation.
type InterpolationContext struct {
	Var      map[string]string // chain-level variables (replaces Trigger)
	Previous *StepResult
	Steps    map[int]*StepResult
}

var templatePattern = regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`)

// Interpolate replaces template expressions in a string.
// Supported expressions:
//   - {{ var.key }} - Value from chain-level variables
//   - {{ previous.inputs.key }} - Value from previous step's inputs
//   - {{ steps.N.inputs.key }} - Value from step N's inputs (0-indexed)
func Interpolate(template string, ctx *InterpolationContext) (string, error) {
	if ctx == nil {
		return template, nil
	}

	result := templatePattern.ReplaceAllStringFunc(template, func(match string) string {
		expr := strings.TrimSpace(match[2 : len(match)-2])
		parts := strings.Split(expr, ".")

		if len(parts) < 2 {
			return match
		}

		switch parts[0] {
		case "var":
			if len(parts) >= 2 && ctx.Var != nil {
				key := strings.Join(parts[1:], ".")
				if val, ok := ctx.Var[key]; ok {
					return val
				}
			}
		case "previous":
			if ctx.Previous != nil && len(parts) >= 3 && parts[1] == "inputs" {
				key := strings.Join(parts[2:], ".")
				if val, ok := ctx.Previous.Inputs[key]; ok {
					return val
				}
			}
		case "steps":
			if ctx.Steps != nil && len(parts) >= 4 && parts[2] == "inputs" {
				var stepNum int
				if parseStepIndex(parts[1], &stepNum) {
					if step, ok := ctx.Steps[stepNum]; ok {
						key := strings.Join(parts[3:], ".")
						if val, ok := step.Inputs[key]; ok {
							return val
						}
					}
				}
			}
		}

		return match
	})

	return result, nil
}

func parseStepIndex(s string, n *int) bool {
	if len(s) == 0 {
		return false
	}
	num := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
		num = num*10 + int(c-'0')
	}
	*n = num
	return true
}

// InterpolateInputs interpolates all values in an input map.
func InterpolateInputs(inputs map[string]string, ctx *InterpolationContext) (map[string]string, error) {
	result := make(map[string]string, len(inputs))
	for key, value := range inputs {
		interpolated, err := Interpolate(value, ctx)
		if err != nil {
			return nil, err
		}
		result[key] = interpolated
	}
	return result, nil
}
