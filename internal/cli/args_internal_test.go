package cli

import (
	"errors"
	"testing"
)

// TestParseRunID_AcceptsFlagsOnEitherSideOfTheRunID guards the argument order a
// caller naturally writes. Go's flag package stops parsing at the first
// positional, so a flag written after the run ID was silently dropped.
func TestParseRunID_AcceptsFlagsOnEitherSideOfTheRunID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantStep   int
		wantErrors bool
	}{
		{"flags first", []string{"--errors-only", "--step", "2", "99"}, 2, true},
		{"flags last", []string{"99", "--errors-only", "--step", "2"}, 2, true},
		{"flags around", []string{"--errors-only", "99", "--step", "2"}, 2, true},
		{"equals form", []string{"99", "--step=2", "--errors-only"}, 2, true},
		{"no flags", []string{"99"}, -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fs := newFlagSet("logs")

			var (
				step       int
				errorsOnly bool
			)

			fs.IntVar(&step, "step", -1, "")
			fs.BoolVar(&errorsOnly, "errors-only", false, "")

			runID, err := parseRunID(fs, tt.args)
			if err != nil {
				t.Fatalf("parseRunID(%v): %v", tt.args, err)
			}

			if runID != 99 {
				t.Errorf("run ID is %d, want 99", runID)
			}

			if step != tt.wantStep {
				t.Errorf("--step is %d, want %d", step, tt.wantStep)
			}

			if errorsOnly != tt.wantErrors {
				t.Errorf("--errors-only is %v, want %v", errorsOnly, tt.wantErrors)
			}
		})
	}
}

func TestParseRunID_RejectsBadArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{"no run ID", []string{"--errors-only"}},
		{"not a number", []string{"latest"}},
		{"two run IDs", []string{"99", "100"}},
		{"unknown flag", []string{"99", "--nope"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fs := newFlagSet("logs")
			fs.Bool("errors-only", false, "")

			if _, err := parseRunID(fs, tt.args); !errors.Is(err, ErrUsage) {
				t.Errorf("parseRunID(%v) = %v, want a usage error", tt.args, err)
			}
		})
	}
}
