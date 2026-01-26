package rule

import (
	"testing"
)

func TestParseValidationComment(t *testing.T) {
	tests := []struct {
		name      string
		comment   string
		wantRule  *ValidationRule
		wantError bool
	}{
		{
			name:     "non-validation comment",
			comment:  "# Just a regular comment",
			wantRule: nil,
		},
		{
			name:     "required rule",
			comment:  "# lazydispatch:validate:required",
			wantRule: &ValidationRule{Type: RuleRequired},
		},
		{
			name:     "required rule with spaces",
			comment:  "  # lazydispatch:validate:required  ",
			wantRule: &ValidationRule{Type: RuleRequired},
		},
		{
			name:     "regex rule",
			comment:  "# lazydispatch:validate:regex:^v\\d+\\.\\d+\\.\\d+$",
			wantRule: &ValidationRule{Type: RuleRegex, Pattern: "^v\\d+\\.\\d+\\.\\d+$"},
		},
		{
			name:      "regex rule without pattern",
			comment:   "# lazydispatch:validate:regex:",
			wantError: true,
		},
		{
			name:      "invalid regex pattern",
			comment:   "# lazydispatch:validate:regex:[invalid",
			wantError: true,
		},
		{
			name:     "range rule",
			comment:  "# lazydispatch:validate:range:1024-65535",
			wantRule: &ValidationRule{Type: RuleRange, Min: 1024, Max: 65535},
		},
		{
			name:      "invalid range format",
			comment:   "# lazydispatch:validate:range:invalid",
			wantError: true,
		},
		{
			name:      "range with min > max",
			comment:   "# lazydispatch:validate:range:100-50",
			wantError: true,
		},
		{
			name:     "prefix rule",
			comment:  "# lazydispatch:validate:prefix:release-",
			wantRule: &ValidationRule{Type: RulePrefix, Pattern: "release-"},
		},
		{
			name:      "prefix rule without value",
			comment:   "# lazydispatch:validate:prefix:",
			wantError: true,
		},
		{
			name:     "suffix rule",
			comment:  "# lazydispatch:validate:suffix:.json",
			wantRule: &ValidationRule{Type: RuleSuffix, Pattern: ".json"},
		},
		{
			name:      "suffix rule without value",
			comment:   "# lazydispatch:validate:suffix:",
			wantError: true,
		},
		{
			name:     "length rule",
			comment:  "# lazydispatch:validate:length:3-50",
			wantRule: &ValidationRule{Type: RuleLength, Min: 3, Max: 50},
		},
		{
			name:     "unknown rule type",
			comment:  "# lazydispatch:validate:unknown:value",
			wantRule: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := ParseValidationComment(tt.comment)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantRule == nil {
				if rule != nil {
					t.Errorf("expected nil rule, got %+v", rule)
				}

				return
			}

			if rule == nil {
				t.Error("expected rule, got nil")
				return
			}

			if rule.Type != tt.wantRule.Type {
				t.Errorf("type = %v, want %v", rule.Type, tt.wantRule.Type)
			}

			if rule.Pattern != tt.wantRule.Pattern {
				t.Errorf("pattern = %q, want %q", rule.Pattern, tt.wantRule.Pattern)
			}

			if rule.Min != tt.wantRule.Min {
				t.Errorf("min = %d, want %d", rule.Min, tt.wantRule.Min)
			}

			if rule.Max != tt.wantRule.Max {
				t.Errorf("max = %d, want %d", rule.Max, tt.wantRule.Max)
			}
		})
	}
}

func TestParseValidationComments(t *testing.T) {
	comments := []string{
		"# lazydispatch:validate:required",
		"# Some other comment",
		"# lazydispatch:validate:prefix:v",
	}

	rules, err := ParseValidationComments(comments)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}

	if rules[0].Type != RuleRequired {
		t.Errorf("first rule type = %v, want %v", rules[0].Type, RuleRequired)
	}

	if rules[1].Type != RulePrefix {
		t.Errorf("second rule type = %v, want %v", rules[1].Type, RulePrefix)
	}
}

func TestValidateValue(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		rules      []ValidationRule
		wantErrors int
	}{
		{
			name:       "no rules",
			value:      "anything",
			rules:      nil,
			wantErrors: 0,
		},
		{
			name:       "required with value",
			value:      "something",
			rules:      []ValidationRule{{Type: RuleRequired}},
			wantErrors: 0,
		},
		{
			name:       "required without value",
			value:      "",
			rules:      []ValidationRule{{Type: RuleRequired}},
			wantErrors: 1,
		},
		{
			name:       "required with whitespace only",
			value:      "   ",
			rules:      []ValidationRule{{Type: RuleRequired}},
			wantErrors: 1,
		},
		{
			name:       "regex match",
			value:      "v1.2.3",
			rules:      []ValidationRule{{Type: RuleRegex, Pattern: "^v\\d+\\.\\d+\\.\\d+$"}},
			wantErrors: 0,
		},
		{
			name:       "regex no match",
			value:      "1.2.3",
			rules:      []ValidationRule{{Type: RuleRegex, Pattern: "^v\\d+\\.\\d+\\.\\d+$"}},
			wantErrors: 1,
		},
		{
			name:       "range valid",
			value:      "8080",
			rules:      []ValidationRule{{Type: RuleRange, Min: 1024, Max: 65535}},
			wantErrors: 0,
		},
		{
			name:       "range below min",
			value:      "80",
			rules:      []ValidationRule{{Type: RuleRange, Min: 1024, Max: 65535}},
			wantErrors: 1,
		},
		{
			name:       "range above max",
			value:      "70000",
			rules:      []ValidationRule{{Type: RuleRange, Min: 1024, Max: 65535}},
			wantErrors: 1,
		},
		{
			name:       "range not a number",
			value:      "abc",
			rules:      []ValidationRule{{Type: RuleRange, Min: 1, Max: 100}},
			wantErrors: 1,
		},
		{
			name:       "range empty value",
			value:      "",
			rules:      []ValidationRule{{Type: RuleRange, Min: 1, Max: 100}},
			wantErrors: 0,
		},
		{
			name:       "prefix match",
			value:      "release-1.0",
			rules:      []ValidationRule{{Type: RulePrefix, Pattern: "release-"}},
			wantErrors: 0,
		},
		{
			name:       "prefix no match",
			value:      "feature-1.0",
			rules:      []ValidationRule{{Type: RulePrefix, Pattern: "release-"}},
			wantErrors: 1,
		},
		{
			name:       "prefix empty value",
			value:      "",
			rules:      []ValidationRule{{Type: RulePrefix, Pattern: "release-"}},
			wantErrors: 0,
		},
		{
			name:       "suffix match",
			value:      "config.json",
			rules:      []ValidationRule{{Type: RuleSuffix, Pattern: ".json"}},
			wantErrors: 0,
		},
		{
			name:       "suffix no match",
			value:      "config.yaml",
			rules:      []ValidationRule{{Type: RuleSuffix, Pattern: ".json"}},
			wantErrors: 1,
		},
		{
			name:       "length valid",
			value:      "hello",
			rules:      []ValidationRule{{Type: RuleLength, Min: 3, Max: 10}},
			wantErrors: 0,
		},
		{
			name:       "length too short",
			value:      "hi",
			rules:      []ValidationRule{{Type: RuleLength, Min: 3, Max: 10}},
			wantErrors: 1,
		},
		{
			name:       "length too long",
			value:      "hello world!",
			rules:      []ValidationRule{{Type: RuleLength, Min: 3, Max: 10}},
			wantErrors: 1,
		},
		{
			name:  "multiple rules all pass",
			value: "release-v1.0.0",
			rules: []ValidationRule{
				{Type: RuleRequired},
				{Type: RulePrefix, Pattern: "release-"},
				{Type: RuleLength, Min: 5, Max: 50},
			},
			wantErrors: 0,
		},
		{
			name:  "multiple rules some fail",
			value: "feature-v1.0.0",
			rules: []ValidationRule{
				{Type: RuleRequired},
				{Type: RulePrefix, Pattern: "release-"},
				{Type: RuleLength, Min: 5, Max: 50},
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateValue(tt.value, tt.rules)

			if len(errors) != tt.wantErrors {
				t.Errorf("ValidateValue() errors = %d, want %d; errors: %v", len(errors), tt.wantErrors, errors)
			}
		})
	}
}

func TestParseRange(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMin   int
		wantMax   int
		wantError bool
	}{
		{
			name:    "valid range",
			input:   "1-100",
			wantMin: 1,
			wantMax: 100,
		},
		{
			name:    "range with spaces",
			input:   "1 - 100",
			wantMin: 1,
			wantMax: 100,
		},
		{
			name:      "invalid format",
			input:     "100",
			wantError: true,
		},
		{
			name:      "non-numeric min",
			input:     "abc-100",
			wantError: true,
		},
		{
			name:      "non-numeric max",
			input:     "1-abc",
			wantError: true,
		},
		{
			name:      "min greater than max",
			input:     "100-1",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minVal, maxVal, err := parseRange(tt.input)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if minVal != tt.wantMin {
				t.Errorf("min = %d, want %d", minVal, tt.wantMin)
			}

			if maxVal != tt.wantMax {
				t.Errorf("max = %d, want %d", maxVal, tt.wantMax)
			}
		})
	}
}
