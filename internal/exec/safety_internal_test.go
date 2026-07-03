package exec

import (
	"testing"
)

func TestRealExecutor_SafetyCheck_BlocksMutations(t *testing.T) {
	executor := NewRealExecutor()

	tests := []struct {
		name        string
		command     string
		args        []string
		shouldPanic bool
	}{
		{
			name:        "blocks gh workflow run",
			command:     "gh",
			args:        []string{"workflow", "run", "test.yml"},
			shouldPanic: true,
		},
		{
			name:        "blocks gh issue create",
			command:     "gh",
			args:        []string{"issue", "create", "--title", "test"},
			shouldPanic: true,
		},
		{
			name:        "blocks gh pr create",
			command:     "gh",
			args:        []string{"pr", "create", "--title", "test"},
			shouldPanic: true,
		},
		{
			name:        "blocks gh run cancel",
			command:     "gh",
			args:        []string{"run", "cancel", "123"},
			shouldPanic: true,
		},
		{
			name:        "allows gh run view",
			command:     "gh",
			args:        []string{"run", "view", "123"},
			shouldPanic: false,
		},
		{
			name:        "allows gh run list",
			command:     "gh",
			args:        []string{"run", "list"},
			shouldPanic: false,
		},
		{
			name:        "allows gh api (read-only)",
			command:     "gh",
			args:        []string{"api", "repos/owner/repo/actions/runs/123"},
			shouldPanic: false,
		},
		{
			name:        "allows non-gh commands",
			command:     "echo",
			args:        []string{"hello"},
			shouldPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.shouldPanic && r == nil {
					t.Errorf("expected panic for command: %s %v", tt.command, tt.args)
				}

				if !tt.shouldPanic && r != nil {
					t.Errorf("unexpected panic for command: %s %v: %v", tt.command, tt.args, r)
				}
			}()

			// This will panic if it's a mutation command
			_, _, _ = executor.Execute(tt.command, tt.args...)
		})
	}
}

func TestIsMutationCommand(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		args       []string
		isMutation bool
	}{
		{
			name:       "gh workflow run is mutation",
			command:    "gh",
			args:       []string{"workflow", "run", "test.yml"},
			isMutation: true,
		},
		{
			name:       "gh run view is read-only",
			command:    "gh",
			args:       []string{"run", "view", "123", "--log"},
			isMutation: false,
		},
		{
			name:       "gh run watch is read-only",
			command:    "gh",
			args:       []string{"run", "watch"},
			isMutation: false,
		},
		{
			name:       "gh run cancel is mutation",
			command:    "gh",
			args:       []string{"run", "cancel", "123"},
			isMutation: true,
		},
		{
			name:       "gh api is read-only",
			command:    "gh",
			args:       []string{"api", "repos/owner/repo/actions/runs"},
			isMutation: false,
		},
		{
			name:       "non-gh command is safe",
			command:    "echo",
			args:       []string{"hello"},
			isMutation: false,
		},
		{
			name:       "gh pr create is mutation",
			command:    "gh",
			args:       []string{"pr", "create"},
			isMutation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMutationCommand(tt.command, tt.args)
			if result != tt.isMutation {
				t.Errorf("isMutationCommand(%s, %v) = %v, want %v",
					tt.command, tt.args, result, tt.isMutation)
			}
		})
	}
}
