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

// minExprParts is the fewest dot-separated segments a template expression can have
// (e.g. "var.key"); anything shorter is left unresolved.
const minExprParts = 2

// decimalBase is used to accumulate digit characters into an integer.
const decimalBase = 10

// Interpolate replaces template expressions in a string.
// Supported expressions:
//   - {{ var.key }} - Value from chain-level variables
//   - {{ previous.inputs.key }} - Value from previous step's inputs
//   - {{ steps.N.inputs.key }} - Value from step N's inputs (0-indexed)
//
//nolint:unparam // error is part of the public API for forward compatibility (e.g. future strict-mode validation)
func Interpolate(template string, ctx *InterpolationContext) (string, error) {
	if ctx == nil {
		return template, nil
	}

	result := templatePattern.ReplaceAllStringFunc(template, func(match string) string {
		expr := strings.TrimSpace(match[2 : len(match)-2])
		parts := strings.Split(expr, ".")

		if len(parts) < minExprParts {
			return match
		}

		switch parts[0] {
		case "var":
			if val, ok := resolveVarExpr(ctx, parts); ok {
				return val
			}
		case "previous":
			if val, ok := resolvePreviousExpr(ctx, parts); ok {
				return val
			}
		case "steps":
			if val, ok := resolveStepsExpr(ctx, parts); ok {
				return val
			}
		}

		return match
	})

	return result, nil
}

// resolveVarExpr resolves a "var.key" expression against chain-level variables.
func resolveVarExpr(ctx *InterpolationContext, parts []string) (string, bool) {
	if len(parts) < 2 || ctx.Var == nil {
		return "", false
	}

	key := strings.Join(parts[1:], ".")
	val, ok := ctx.Var[key]

	return val, ok
}

// resolvePreviousExpr resolves a "previous.inputs.key" expression against the previous step's inputs.
func resolvePreviousExpr(ctx *InterpolationContext, parts []string) (string, bool) {
	if ctx.Previous == nil || len(parts) < 3 || parts[1] != "inputs" {
		return "", false
	}

	key := strings.Join(parts[2:], ".")
	val, ok := ctx.Previous.Inputs[key]

	return val, ok
}

// resolveStepsExpr resolves a "steps.N.inputs.key" expression against a specific step's inputs.
func resolveStepsExpr(ctx *InterpolationContext, parts []string) (string, bool) {
	if ctx.Steps == nil || len(parts) < 4 || parts[2] != "inputs" {
		return "", false
	}

	var stepNum int
	if !parseStepIndex(parts[1], &stepNum) {
		return "", false
	}

	step, ok := ctx.Steps[stepNum]
	if !ok {
		return "", false
	}

	key := strings.Join(parts[3:], ".")
	val, ok := step.Inputs[key]

	return val, ok
}

func parseStepIndex(s string, n *int) bool {
	if s == "" {
		return false
	}

	num := 0

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}

		num = num*decimalBase + int(c-'0')
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
