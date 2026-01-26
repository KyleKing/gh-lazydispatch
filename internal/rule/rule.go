// Package rule provides parsing and validation of workflow input validation rules.
package rule

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// RuleType represents the type of validation rule.
type RuleType int

const (
	RuleRegex RuleType = iota
	RuleRange
	RuleRequired
	RulePrefix
	RuleSuffix
	RuleLength
)

// ValidationRule represents a single validation rule parsed from YAML comments.
type ValidationRule struct {
	Type    RuleType
	Pattern string
	Min     int
	Max     int
}

const validationPrefix = "lazydispatch:validate:"

// ParseValidationComment parses a single comment line for validation rules.
// Returns nil if the comment doesn't contain a validation rule.
func ParseValidationComment(comment string) (*ValidationRule, error) {
	comment = strings.TrimSpace(comment)
	comment = strings.TrimPrefix(comment, "#")
	comment = strings.TrimSpace(comment)

	if !strings.HasPrefix(comment, validationPrefix) {
		return nil, nil
	}

	ruleSpec := strings.TrimPrefix(comment, validationPrefix)

	parts := strings.SplitN(ruleSpec, ":", 2)
	if len(parts) == 0 {
		return nil, nil
	}

	ruleType := parts[0]

	ruleValue := ""
	if len(parts) > 1 {
		ruleValue = parts[1]
	}

	switch ruleType {
	case "regex":
		if ruleValue == "" {
			return nil, errors.New("regex rule requires a pattern")
		}

		if _, err := regexp.Compile(ruleValue); err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}

		return &ValidationRule{Type: RuleRegex, Pattern: ruleValue}, nil

	case "range":
		minVal, maxVal, err := parseRange(ruleValue)
		if err != nil {
			return nil, fmt.Errorf("invalid range: %w", err)
		}

		return &ValidationRule{Type: RuleRange, Min: minVal, Max: maxVal}, nil

	case "required":
		return &ValidationRule{Type: RuleRequired}, nil

	case "prefix":
		if ruleValue == "" {
			return nil, errors.New("prefix rule requires a value")
		}

		return &ValidationRule{Type: RulePrefix, Pattern: ruleValue}, nil

	case "suffix":
		if ruleValue == "" {
			return nil, errors.New("suffix rule requires a value")
		}

		return &ValidationRule{Type: RuleSuffix, Pattern: ruleValue}, nil

	case "length":
		minVal, maxVal, err := parseRange(ruleValue)
		if err != nil {
			return nil, fmt.Errorf("invalid length: %w", err)
		}

		return &ValidationRule{Type: RuleLength, Min: minVal, Max: maxVal}, nil

	default:
		return nil, nil
	}
}

// ParseValidationComments parses multiple comment lines and returns all valid rules.
func ParseValidationComments(comments []string) ([]ValidationRule, error) {
	var rules []ValidationRule

	for _, comment := range comments {
		rule, err := ParseValidationComment(comment)
		if err != nil {
			return nil, err
		}

		if rule != nil {
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

func parseRange(s string) (int, int, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return 0, 0, errors.New("expected format: min-max")
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
		return 0, 0, errors.New("min must be less than or equal to max")
	}

	return minVal, maxVal, nil
}
