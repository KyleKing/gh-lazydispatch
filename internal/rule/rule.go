// Package rule provides parsing and validation of workflow input validation rules.
package rule

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Type represents the type of validation rule.
type Type int

// Validation rule types.
const (
	RuleRegex Type = iota
	RuleRange
	RuleRequired
	RulePrefix
	RuleSuffix
	RuleLength
)

// ValidationRule represents a single validation rule parsed from YAML comments.
type ValidationRule struct {
	Pattern string
	Type    Type
	Min     int
	Max     int
}

const validationPrefix = "lazydispatch:validate:"

// Errors returned while parsing validation rules.
var (
	ErrRegexRuleMissingPattern = errors.New("regex rule requires a pattern")
	ErrPrefixRuleMissingValue  = errors.New("prefix rule requires a value")
	ErrSuffixRuleMissingValue  = errors.New("suffix rule requires a value")
	ErrInvalidRangeFormat      = errors.New("expected format: min-max")
	ErrRangeMinGreaterThanMax  = errors.New("min must be less than or equal to max")
)

// ruleSpecMaxParts caps splitting "type:value" into at most a type and a value.
const ruleSpecMaxParts = 2

// rangeParts is the expected number of "min-max" segments in a range spec.
const rangeParts = 2

// ParseValidationComment parses a single comment line for validation rules.
// The ok return is false if the comment doesn't contain a validation rule.
func ParseValidationComment(comment string) (rule *ValidationRule, ok bool, err error) {
	comment = strings.TrimSpace(comment)
	comment = strings.TrimPrefix(comment, "#")
	comment = strings.TrimSpace(comment)

	if !strings.HasPrefix(comment, validationPrefix) {
		return nil, false, nil
	}

	ruleSpec := strings.TrimPrefix(comment, validationPrefix)

	parts := strings.SplitN(ruleSpec, ":", ruleSpecMaxParts)
	if len(parts) == 0 {
		return nil, false, nil
	}

	ruleType := parts[0]

	ruleValue := ""
	if len(parts) > 1 {
		ruleValue = parts[1]
	}

	switch ruleType {
	case "regex":
		if ruleValue == "" {
			return nil, false, ErrRegexRuleMissingPattern
		}

		if _, err := regexp.Compile(ruleValue); err != nil {
			return nil, false, fmt.Errorf("invalid regex pattern: %w", err)
		}

		return &ValidationRule{Type: RuleRegex, Pattern: ruleValue}, true, nil

	case "range":
		minVal, maxVal, err := parseRange(ruleValue)
		if err != nil {
			return nil, false, fmt.Errorf("invalid range: %w", err)
		}

		return &ValidationRule{Type: RuleRange, Min: minVal, Max: maxVal}, true, nil

	case "required":
		return &ValidationRule{Type: RuleRequired}, true, nil

	case "prefix":
		if ruleValue == "" {
			return nil, false, ErrPrefixRuleMissingValue
		}

		return &ValidationRule{Type: RulePrefix, Pattern: ruleValue}, true, nil

	case "suffix":
		if ruleValue == "" {
			return nil, false, ErrSuffixRuleMissingValue
		}

		return &ValidationRule{Type: RuleSuffix, Pattern: ruleValue}, true, nil

	case "length":
		minVal, maxVal, err := parseRange(ruleValue)
		if err != nil {
			return nil, false, fmt.Errorf("invalid length: %w", err)
		}

		return &ValidationRule{Type: RuleLength, Min: minVal, Max: maxVal}, true, nil

	default:
		return nil, false, nil
	}
}

// ParseValidationComments parses multiple comment lines and returns all valid rules.
func ParseValidationComments(comments []string) ([]ValidationRule, error) {
	var rules []ValidationRule

	for _, comment := range comments {
		rule, ok, err := ParseValidationComment(comment)
		if err != nil {
			return nil, err
		}

		if ok {
			rules = append(rules, *rule)
		}
	}

	return rules, nil
}

// ValidateValue validates a value against a set of rules.
// Returns a slice of error messages for any failed validations.
func ValidateValue(value string, rules []ValidationRule) []string {
	var validationErrs []string

	for _, r := range rules {
		if errMsg := validateRule(value, r); errMsg != "" {
			validationErrs = append(validationErrs, errMsg)
		}
	}

	return validationErrs
}

func validateRule(value string, r ValidationRule) string {
	switch r.Type {
	case RuleRequired:
		if strings.TrimSpace(value) == "" {
			return "value is required"
		}

	case RuleRegex:
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return "invalid regex pattern: " + r.Pattern
		}

		if !re.MatchString(value) {
			return "must match pattern: " + r.Pattern
		}

	case RuleRange:
		if value == "" {
			return ""
		}

		num, err := strconv.Atoi(value)
		if err != nil {
			return "must be a number"
		}

		if num < r.Min || num > r.Max {
			return fmt.Sprintf("must be between %d and %d", r.Min, r.Max)
		}

	case RulePrefix:
		if value != "" && !strings.HasPrefix(value, r.Pattern) {
			return "must start with: " + r.Pattern
		}

	case RuleSuffix:
		if value != "" && !strings.HasSuffix(value, r.Pattern) {
			return "must end with: " + r.Pattern
		}

	case RuleLength:
		length := len(value)
		if length < r.Min || length > r.Max {
			return fmt.Sprintf("length must be between %d and %d", r.Min, r.Max)
		}
	}

	return ""
}

// parseRange returns (min, max, err).
//
//nolint:gocritic // unnamedResult wants named returns, but nonamedreturns forbids them
func parseRange(s string) (int, int, error) {
	parts := strings.Split(s, "-")
	if len(parts) != rangeParts {
		return 0, 0, ErrInvalidRangeFormat
	}

	minVal, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid min value: %w", err)
	}

	maxVal, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid max value: %w", err)
	}

	if minVal > maxVal {
		return 0, 0, ErrRangeMinGreaterThanMax
	}

	return minVal, maxVal, nil
}
