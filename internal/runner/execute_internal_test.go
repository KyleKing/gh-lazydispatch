package runner

import (
	"errors"
	"strings"
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

func TestBuildArgs(t *testing.T) {
	tests := []struct {
		name         string
		cfg          RunConfig
		wantContains []string
		wantExcludes []string
		wantLen      int
	}{
		{
			name: "basic",
			cfg: RunConfig{
				Workflow: "deploy.yml",
			},
			wantContains: []string{"workflow", "run", "deploy.yml"},
			wantLen:      3,
		},
		{
			name: "with branch",
			cfg: RunConfig{
				Workflow: "deploy.yml",
				Branch:   "main",
			},
			wantContains: []string{"workflow", "run", "deploy.yml", "--ref", "main"},
		},
		{
			name: "with inputs",
			cfg: RunConfig{
				Workflow: "deploy.yml",
				Inputs: map[string]string{
					"environment": "production",
					"dry_run":     "false",
				},
			},
			wantContains: []string{"workflow", "run", "deploy.yml", "-f", "environment=production", "-f", "dry_run=false"},
		},
		{
			name: "empty inputs skipped",
			cfg: RunConfig{
				Workflow: "deploy.yml",
				Inputs: map[string]string{
					"environment": "production",
					"empty":       "",
				},
			},
			wantContains: []string{"environment=production"},
			wantExcludes: []string{"empty="},
		},
		{
			name: "all options",
			cfg: RunConfig{
				Workflow: "ci.yml",
				Branch:   "feature/test",
				Inputs: map[string]string{
					"env":     "staging",
					"verbose": "true",
				},
			},
			wantContains: []string{"workflow", "run", "ci.yml", "--ref", "feature/test", "-f", "env=staging", "-f", "verbose=true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := BuildArgs(tt.cfg)

			if tt.wantLen > 0 && len(args) != tt.wantLen {
				t.Errorf("BuildArgs() length = %d, want %d: %v", len(args), tt.wantLen, args)
			}

			argsStr := strings.Join(args, " ")
			for _, want := range tt.wantContains {
				if !strings.Contains(argsStr, want) {
					t.Errorf("BuildArgs() missing %q in: %v", want, args)
				}
			}

			for _, exclude := range tt.wantExcludes {
				if strings.Contains(argsStr, exclude) {
					t.Errorf("BuildArgs() should not contain %q in: %v", exclude, args)
				}
			}
		})
	}
}

func TestFormatCommand(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantPrefix   string
		wantContains []string
	}{
		{
			name:         "basic command",
			args:         []string{"workflow", "run", "deploy.yml", "--ref", "main", "-f", "env=prod"},
			wantPrefix:   "gh workflow run",
			wantContains: []string{"deploy.yml", "main", "env=prod"},
		},
		{
			name:         "quotes special chars",
			args:         []string{"workflow", "run", "deploy.yml", "-f", "message=hello world"},
			wantPrefix:   "gh workflow run",
			wantContains: []string{"\"message=hello world\""},
		},
		{
			name:         "quotes args with equals",
			args:         []string{"workflow", "run", "test.yml", "-f", "key=value"},
			wantPrefix:   "gh workflow run",
			wantContains: []string{"\"key=value\""},
		},
		{
			name:         "no special chars",
			args:         []string{"workflow", "run", "simple.yml"},
			wantPrefix:   "gh workflow run",
			wantContains: []string{"simple.yml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := FormatCommand(tt.args)

			if !strings.HasPrefix(cmd, tt.wantPrefix) {
				t.Errorf("FormatCommand() prefix = %q, want %q", cmd, tt.wantPrefix)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(cmd, want) {
					t.Errorf("FormatCommand() missing %q in: %s", want, cmd)
				}
			}
		})
	}
}

func TestDryRun(t *testing.T) {
	tests := []struct {
		name         string
		cfg          RunConfig
		wantContains []string
	}{
		{
			name: "full config",
			cfg: RunConfig{
				Workflow: "deploy.yml",
				Branch:   "main",
				Inputs:   map[string]string{"env": "prod"},
			},
			wantContains: []string{"gh workflow run", "deploy.yml", "main", "env=prod"},
		},
		{
			name: "minimal config",
			cfg: RunConfig{
				Workflow: "test.yml",
			},
			wantContains: []string{"gh workflow run", "test.yml"},
		},
		{
			name: "with multiple inputs",
			cfg: RunConfig{
				Workflow: "ci.yml",
				Branch:   "feature",
				Inputs: map[string]string{
					"debug":   "true",
					"verbose": "1",
				},
			},
			wantContains: []string{"gh workflow run", "ci.yml", "feature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := DryRun(tt.cfg)

			if cmd == "" {
				t.Error("DryRun() returned empty string")
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(cmd, want) {
					t.Errorf("DryRun() missing %q in: %s", want, cmd)
				}
			}
		})
	}
}

// mockCommand tracks a command execution.
type mockCommand struct {
	name string
	args []string
}

// mockCommandExecutor is a test double for CommandExecutor.
type mockCommandExecutor struct {
	executedCommands []mockCommand
	errorOnCommand   int
	commandCounter   int
	errToReturn      error
}

func (m *mockCommandExecutor) Execute(name string, args ...string) error {
	m.executedCommands = append(m.executedCommands, mockCommand{
		name: name,
		args: args,
	})

	if m.errorOnCommand >= 0 && m.commandCounter == m.errorOnCommand {
		m.commandCounter++
		return m.errToReturn
	}

	m.commandCounter++

	return nil
}

// mockGitHubClient is a test double for GitHubClient.
type mockGitHubClient struct {
	run *github.WorkflowRun
	err error
}

func (m *mockGitHubClient) GetLatestRun(_ string) (*github.WorkflowRun, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.run, nil
}

// mockRepositoryDetector is a test double for RepositoryDetector.
type mockRepositoryDetector struct {
	repo Repository
	err  error
}

func (m *mockRepositoryDetector) Current() (Repository, error) {
	if m.err != nil {
		return Repository{}, m.err
	}

	return m.repo, nil
}

func TestExecuteWithExecutor(t *testing.T) {
	tests := []struct {
		name           string
		cfg            RunConfig
		errorOnCommand int
		errToReturn    error
		expectError    bool
		wantCommands   int
	}{
		{
			name: "basic execution without watch",
			cfg: RunConfig{
				Workflow: "deploy.yml",
				Branch:   "main",
			},
			errorOnCommand: -1,
			expectError:    false,
			wantCommands:   1,
		},
		{
			name: "execution with inputs",
			cfg: RunConfig{
				Workflow: "ci.yml",
				Branch:   "feature",
				Inputs: map[string]string{
					"env":     "staging",
					"verbose": "true",
				},
			},
			errorOnCommand: -1,
			expectError:    false,
			wantCommands:   1,
		},
		{
			name: "execution with watch flag",
			cfg: RunConfig{
				Workflow: "test.yml",
				Branch:   "main",
				Watch:    true,
			},
			errorOnCommand: -1,
			expectError:    false,
			wantCommands:   2,
		},
		{
			name: "command execution fails",
			cfg: RunConfig{
				Workflow: "deploy.yml",
			},
			errorOnCommand: 0,
			errToReturn:    errors.New("command failed"),
			expectError:    true,
			wantCommands:   1,
		},
		{
			name: "watch command fails",
			cfg: RunConfig{
				Workflow: "test.yml",
				Watch:    true,
			},
			errorOnCommand: 1,
			errToReturn:    errors.New("watch failed"),
			expectError:    true,
			wantCommands:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCommandExecutor{
				errorOnCommand: tt.errorOnCommand,
				errToReturn:    tt.errToReturn,
			}

			err := ExecuteWithExecutor(tt.cfg, mock)

			if (err != nil) != tt.expectError {
				t.Errorf("ExecuteWithExecutor() error = %v, expectError %v", err, tt.expectError)
			}

			if len(mock.executedCommands) != tt.wantCommands {
				t.Errorf("ExecuteWithExecutor() executed %d commands, want %d", len(mock.executedCommands), tt.wantCommands)
			}

			if len(mock.executedCommands) > 0 {
				firstCmd := mock.executedCommands[0]
				if firstCmd.name != "gh" {
					t.Errorf("ExecuteWithExecutor() first command name = %q, want %q", firstCmd.name, "gh")
				}
			}

			if tt.cfg.Watch && len(mock.executedCommands) == 2 {
				watchCmd := mock.executedCommands[1]
				if watchCmd.name != "gh" || len(watchCmd.args) < 2 || watchCmd.args[0] != "run" || watchCmd.args[1] != "watch" {
					t.Errorf("ExecuteWithExecutor() watch command = %v, want [gh run watch]", watchCmd)
				}
			}
		})
	}
}

func TestExecuteAndGetRunIDWithExecutor(t *testing.T) {
	tests := []struct {
		name           string
		cfg            RunConfig
		mockRun        *github.WorkflowRun
		mockRunErr     error
		execError      bool
		errorOnCommand int
		expectError    bool
		expectRunID    int64
	}{
		{
			name: "successful execution and retrieval",
			cfg: RunConfig{
				Workflow: "deploy.yml",
				Branch:   "main",
			},
			mockRun: &github.WorkflowRun{
				ID: 12345,
			},
			mockRunErr:     nil,
			errorOnCommand: -1,
			expectError:    false,
			expectRunID:    12345,
		},
		{
			name: "command execution fails",
			cfg: RunConfig{
				Workflow: "test.yml",
			},
			mockRun:        nil,
			mockRunErr:     nil,
			errorOnCommand: 0,
			expectError:    true,
			expectRunID:    0,
		},
		{
			name: "GetLatestRun API error",
			cfg: RunConfig{
				Workflow: "ci.yml",
				Branch:   "feature",
			},
			mockRun:        nil,
			mockRunErr:     errors.New("API error"),
			errorOnCommand: -1,
			expectError:    true,
			expectRunID:    0,
		},
		{
			name: "no run found",
			cfg: RunConfig{
				Workflow: "deploy.yml",
			},
			mockRun:        nil,
			mockRunErr:     nil,
			errorOnCommand: -1,
			expectError:    true,
			expectRunID:    0,
		},
		{
			name: "with inputs",
			cfg: RunConfig{
				Workflow: "test.yml",
				Branch:   "develop",
				Inputs: map[string]string{
					"env": "test",
				},
			},
			mockRun: &github.WorkflowRun{
				ID: 67890,
			},
			mockRunErr:     nil,
			errorOnCommand: -1,
			expectError:    false,
			expectRunID:    67890,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &mockCommandExecutor{
				errorOnCommand: tt.errorOnCommand,
				errToReturn:    errors.New("command failed"),
			}
			mockClient := &mockGitHubClient{
				run: tt.mockRun,
				err: tt.mockRunErr,
			}

			runID, err := ExecuteAndGetRunIDWithExecutor(tt.cfg, mockClient, mockExec)

			if (err != nil) != tt.expectError {
				t.Errorf("ExecuteAndGetRunIDWithExecutor() error = %v, expectError %v", err, tt.expectError)
			}

			if runID != tt.expectRunID {
				t.Errorf("ExecuteAndGetRunIDWithExecutor() runID = %d, want %d", runID, tt.expectRunID)
			}
		})
	}
}

func TestWatchLatestRunWithExecutor(t *testing.T) {
	tests := []struct {
		name        string
		workflow    string
		execError   bool
		expectError bool
	}{
		{
			name:        "successful watch",
			workflow:    "deploy.yml",
			execError:   false,
			expectError: false,
		},
		{
			name:        "watch command fails",
			workflow:    "test.yml",
			execError:   true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &mockCommandExecutor{
				errorOnCommand: -1,
			}
			if tt.execError {
				mockExec.errorOnCommand = 0
				mockExec.errToReturn = errors.New("watch failed")
			}

			err := watchLatestRunWithExecutor(tt.workflow, mockExec)

			if (err != nil) != tt.expectError {
				t.Errorf("watchLatestRunWithExecutor() error = %v, expectError %v", err, tt.expectError)
			}

			if len(mockExec.executedCommands) != 1 {
				t.Errorf("watchLatestRunWithExecutor() executed %d commands, want 1", len(mockExec.executedCommands))
			}

			if len(mockExec.executedCommands) > 0 {
				cmd := mockExec.executedCommands[0]
				if cmd.name != "gh" {
					t.Errorf("watchLatestRunWithExecutor() command name = %q, want %q", cmd.name, "gh")
				}

				if len(cmd.args) != 2 || cmd.args[0] != "run" || cmd.args[1] != "watch" {
					t.Errorf("watchLatestRunWithExecutor() args = %v, want [run watch]", cmd.args)
				}
			}
		})
	}
}

func TestDetectRepoWithDetector(t *testing.T) {
	tests := []struct {
		name        string
		mockRepo    Repository
		mockErr     error
		expectError bool
		expectRepo  string
	}{
		{
			name: "successful detection",
			mockRepo: Repository{
				Owner: "kyleking",
				Name:  "gh-lazydispatch",
			},
			mockErr:     nil,
			expectError: false,
			expectRepo:  "kyleking/gh-lazydispatch",
		},
		{
			name: "different repo format",
			mockRepo: Repository{
				Owner: "octocat",
				Name:  "hello-world",
			},
			mockErr:     nil,
			expectError: false,
			expectRepo:  "octocat/hello-world",
		},
		{
			name:        "detection failure",
			mockRepo:    Repository{},
			mockErr:     errors.New("not a git repository"),
			expectError: true,
			expectRepo:  "",
		},
		{
			name: "empty repo with error",
			mockRepo: Repository{
				Owner: "",
				Name:  "",
			},
			mockErr:     errors.New("repository not found"),
			expectError: true,
			expectRepo:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRepositoryDetector{
				repo: tt.mockRepo,
				err:  tt.mockErr,
			}

			repo, err := DetectRepoWithDetector(mock)

			if (err != nil) != tt.expectError {
				t.Errorf("DetectRepoWithDetector() error = %v, expectError %v", err, tt.expectError)
			}

			if repo != tt.expectRepo {
				t.Errorf("DetectRepoWithDetector() = %q, want %q", repo, tt.expectRepo)
			}
		})
	}
}
