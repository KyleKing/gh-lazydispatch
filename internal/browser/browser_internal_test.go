package browser

import (
	"runtime"
	"slices"
	"testing"
)

const (
	goosDarwin     = "darwin"
	goosLinux      = "linux"
	goosWindows    = "windows"
	cmdOpen        = "open"
	cmdXdgOpen     = "xdg-open"
	cmdWindowsExe  = "cmd"
	testExampleURL = "https://example.com"
)

// mockCmd is a mock command runner that doesn't actually execute anything.
type mockCmd struct {
	name string
	args []string
	err  error
}

func (m *mockCmd) Start() error {
	return m.err
}

//nolint:paralleltest // swaps the package-level execCommand var; cannot run concurrent with same-var tests
func TestOpen(t *testing.T) {
	// Save original and restore after test
	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	var capturedCmd *mockCmd
	execCommand = func(name string, args ...string) cmdRunner {
		cmd := &mockCmd{name: name, args: args}
		capturedCmd = cmd

		return cmd
	}

	url := testExampleURL
	err := Open(url)
	if err != nil {
		t.Errorf("Open failed: %v", err)
	}

	if capturedCmd == nil {
		t.Fatal("expected command to be executed")
	}

	// Verify correct command based on OS
	switch runtime.GOOS {
	case goosDarwin:
		if capturedCmd.name != cmdOpen {
			t.Errorf("expected command 'open', got '%s'", capturedCmd.name)
		}
		if len(capturedCmd.args) != 1 || capturedCmd.args[0] != url {
			t.Errorf("expected args [%s], got %v", url, capturedCmd.args)
		}
	case goosLinux:
		if capturedCmd.name != cmdXdgOpen {
			t.Errorf("expected command 'xdg-open', got '%s'", capturedCmd.name)
		}
		if len(capturedCmd.args) != 1 || capturedCmd.args[0] != url {
			t.Errorf("expected args [%s], got %v", url, capturedCmd.args)
		}
	case goosWindows:
		if capturedCmd.name != cmdWindowsExe {
			t.Errorf("expected command 'cmd', got '%s'", capturedCmd.name)
		}
		expectedArgs := []string{"/c", "start", url}
		if len(capturedCmd.args) != len(expectedArgs) {
			t.Errorf("expected args %v, got %v", expectedArgs, capturedCmd.args)
		}
	}
}

//nolint:paralleltest // swaps the package-level execCommand var; cannot run concurrent with same-var tests
func TestOpen_InvalidURL(t *testing.T) {
	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	var capturedCmd *mockCmd
	execCommand = func(name string, args ...string) cmdRunner {
		cmd := &mockCmd{name: name, args: args}
		capturedCmd = cmd

		return cmd
	}

	url := "not a valid url"
	err := Open(url)
	if err != nil {
		t.Errorf("Open failed: %v", err)
	}

	if capturedCmd == nil {
		t.Fatal("expected command to be executed")
	}

	// Verify the URL was passed to the command (validation happens at browser level)
	found := false
	for _, arg := range capturedCmd.args {
		if arg == url {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected URL '%s' in args %v", url, capturedCmd.args)
	}
}

//nolint:paralleltest // swaps the package-level execCommand var; cannot run concurrent with same-var tests
func TestOpen_EmptyURL(t *testing.T) {
	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	var capturedCmd *mockCmd
	execCommand = func(name string, args ...string) cmdRunner {
		cmd := &mockCmd{name: name, args: args}
		capturedCmd = cmd

		return cmd
	}

	err := Open("")
	if err != nil {
		t.Errorf("Open failed: %v", err)
	}

	if capturedCmd == nil {
		t.Fatal("expected command to be executed")
	}

	// Verify empty URL is passed to command
	found := false
	for _, arg := range capturedCmd.args {
		if arg == "" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected empty URL in args %v", capturedCmd.args)
	}
}

// Every platform's branch is reachable from the one the tests run on, which is
// the point of taking the OS as an argument.
func TestOpenCommand_NamesEachPlatformsOwnOpener(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos, name string
		args       []string
	}{
		{goos: goosDarwin, name: cmdOpen, args: []string{testExampleURL}},
		{goos: goosLinux, name: cmdXdgOpen, args: []string{testExampleURL}},
		{goos: goosWindows, name: cmdWindowsExe, args: []string{"/c", "start", testExampleURL}},
		{goos: "plan9", name: cmdXdgOpen, args: []string{testExampleURL}},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			t.Parallel()

			name, args := openCommand(tt.goos, testExampleURL)
			if name != tt.name || !slices.Equal(args, tt.args) {
				t.Errorf("%s opens with %s %v, want %s %v", tt.goos, name, args, tt.name, tt.args)
			}
		})
	}
}
