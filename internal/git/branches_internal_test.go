package git

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestParseBranches(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{
			name:   "empty output",
			output: "",
			want:   []string{},
		},
		{
			name:   "single branch",
			output: "  origin/main",
			want:   []string{"main"},
		},
		{
			name: "multiple branches",
			output: `  origin/main
  origin/develop
  origin/feature/test`,
			want: []string{"main", "develop", "feature/test"},
		},
		{
			name: "with HEAD reference",
			output: `  origin/HEAD -> origin/main
  origin/main
  origin/develop`,
			want: []string{"main", "develop"},
		},
		{
			name: "already stripped prefix",
			output: `  main
  develop`,
			want: []string{"main", "develop"},
		},
		{
			name:   "whitespace variations",
			output: "\n  origin/main  \n\n  origin/develop\n",
			want:   []string{"main", "develop"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := _parseBranches(tt.output)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("_parseBranches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeduplicateBranches(t *testing.T) {
	tests := []struct {
		name     string
		branches []string
		want     []string
	}{
		{
			name:     "no duplicates",
			branches: []string{"main", "develop", "feature"},
			want:     []string{"main", "develop", "feature"},
		},
		{
			name:     "with duplicates",
			branches: []string{"main", "develop", "main", "feature", "develop"},
			want:     []string{"main", "develop", "feature"},
		},
		{
			name:     "all duplicates",
			branches: []string{"main", "main", "main"},
			want:     []string{"main"},
		},
		{
			name:     "empty list",
			branches: []string{},
			want:     []string{},
		},
		{
			name:     "single branch",
			branches: []string{"main"},
			want:     []string{"main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := _deduplicateBranches(tt.branches)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("_deduplicateBranches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultBranches(t *testing.T) {
	got := _defaultBranches()

	expectedBranches := []string{"main", "master", "develop"}
	if !reflect.DeepEqual(got, expectedBranches) {
		t.Errorf("_defaultBranches() = %v, want %v", got, expectedBranches)
	}

	if len(got) != 3 {
		t.Errorf("_defaultBranches() length = %d, want 3", len(got))
	}
}

// mockCommandRunner is a test double for CommandRunner.
type mockCommandRunner struct {
	output []byte
	err    error
}

func (m *mockCommandRunner) RunCommand(_ context.Context, _ ...string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.output, nil
}

func TestFetchBranches(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		err            error
		expectedBranch []string
		expectedErr    bool
	}{
		{
			name: "success with multiple branches",
			output: `  origin/main
  origin/develop
  origin/feature/test`,
			err:            nil,
			expectedBranch: []string{"develop", "feature/test", "main"},
			expectedErr:    false,
		},
		{
			name: "HEAD reference filtered",
			output: `  origin/HEAD -> origin/main
  origin/main
  origin/develop`,
			err:            nil,
			expectedBranch: []string{"develop", "main"},
			expectedErr:    false,
		},
		{
			name:           "command error returns defaults",
			output:         "",
			err:            errors.New("git command failed"),
			expectedBranch: []string{"main", "master", "develop"},
			expectedErr:    true,
		},
		{
			name:           "empty output returns defaults",
			output:         "",
			err:            nil,
			expectedBranch: []string{"main", "master", "develop"},
			expectedErr:    false,
		},
		{
			name: "whitespace handling",
			output: `
  origin/main

  origin/develop
`,
			err:            nil,
			expectedBranch: []string{"develop", "main"},
			expectedErr:    false,
		},
		{
			name: "deduplication",
			output: `  origin/main
  origin/main
  origin/develop`,
			err:            nil,
			expectedBranch: []string{"develop", "main"},
			expectedErr:    false,
		},
		{
			name: "feature branches with slashes",
			output: `  origin/feature/auth
  origin/feature/ui/redesign
  origin/main`,
			err:            nil,
			expectedBranch: []string{"feature/auth", "feature/ui/redesign", "main"},
			expectedErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCommandRunner{
				output: []byte(tt.output),
				err:    tt.err,
			}

			got, err := fetchBranchesWithRunner(context.Background(), mock)

			if (err != nil) != tt.expectedErr {
				t.Errorf("fetchBranchesWithRunner() error = %v, expectedErr %v", err, tt.expectedErr)
				return
			}

			if !reflect.DeepEqual(got, tt.expectedBranch) {
				t.Errorf("fetchBranchesWithRunner() = %v, want %v", got, tt.expectedBranch)
			}
		})
	}
}

func TestGetCurrentBranch(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		err            error
		expectedBranch string
	}{
		{
			name:           "normal branch",
			output:         "main",
			err:            nil,
			expectedBranch: "main",
		},
		{
			name:           "feature branch with slashes",
			output:         "feature/auth/oauth",
			err:            nil,
			expectedBranch: "feature/auth/oauth",
		},
		{
			name:           "detached HEAD returns empty",
			output:         "HEAD",
			err:            nil,
			expectedBranch: "",
		},
		{
			name:           "command error returns empty",
			output:         "",
			err:            errors.New("not a git repository"),
			expectedBranch: "",
		},
		{
			name:           "whitespace trimming",
			output:         "  develop  \n",
			err:            nil,
			expectedBranch: "develop",
		},
		{
			name:           "empty output returns empty",
			output:         "",
			err:            nil,
			expectedBranch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCommandRunner{
				output: []byte(tt.output),
				err:    tt.err,
			}

			got := getCurrentBranchWithRunner(context.Background(), mock)

			if got != tt.expectedBranch {
				t.Errorf("getCurrentBranchWithRunner() = %q, want %q", got, tt.expectedBranch)
			}
		})
	}
}

func TestGetDefaultBranch(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		err            error
		expectedBranch string
	}{
		{
			name:           "main extraction",
			output:         "refs/remotes/origin/main",
			err:            nil,
			expectedBranch: "main",
		},
		{
			name:           "master extraction",
			output:         "refs/remotes/origin/master",
			err:            nil,
			expectedBranch: "master",
		},
		{
			name:           "develop extraction",
			output:         "refs/remotes/origin/develop",
			err:            nil,
			expectedBranch: "develop",
		},
		{
			name:           "missing origin HEAD returns empty",
			output:         "",
			err:            errors.New("ref does not exist"),
			expectedBranch: "",
		},
		{
			name:           "whitespace trimming",
			output:         "  refs/remotes/origin/main  \n",
			err:            nil,
			expectedBranch: "main",
		},
		{
			name:           "feature branch",
			output:         "refs/remotes/origin/feature/new",
			err:            nil,
			expectedBranch: "feature/new",
		},
		{
			name:           "empty output returns empty",
			output:         "",
			err:            nil,
			expectedBranch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCommandRunner{
				output: []byte(tt.output),
				err:    tt.err,
			}

			got := getDefaultBranchWithRunner(context.Background(), mock)

			if got != tt.expectedBranch {
				t.Errorf("getDefaultBranchWithRunner() = %q, want %q", got, tt.expectedBranch)
			}
		})
	}
}

func TestFetchBranchesTimeout(t *testing.T) {
	mock := &mockCommandRunner{
		output: []byte("  origin/main"),
		err:    context.DeadlineExceeded,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	branches, err := fetchBranchesWithRunner(ctx, mock)

	if err == nil {
		t.Error("fetchBranchesWithRunner() expected timeout error, got nil")
	}

	if !reflect.DeepEqual(branches, _defaultBranches()) {
		t.Errorf("fetchBranchesWithRunner() on timeout = %v, want default branches", branches)
	}
}
