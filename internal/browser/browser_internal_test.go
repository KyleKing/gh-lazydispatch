package browser

import (
	"runtime"
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

// checkOpenSingleArgCommand runs Open(url) on the given OS (skipping if the
// current runtime.GOOS doesn't match) and asserts it invoked wantCmdName with
// the URL as its sole argument.
func checkOpenSingleArgCommand(t *testing.T, goos, osLabel, wantCmdName string) {
	t.Helper()

	if runtime.GOOS != goos {
		t.Skipf("skipping %s-specific test", osLabel)
	}

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
		t.Errorf("Open on %s failed: %v", osLabel, err)
	}

	if capturedCmd == nil {
		t.Fatal("expected command to be executed")
	}

	if capturedCmd.name != wantCmdName {
		t.Errorf("expected command '%s' on %s, got '%s'", wantCmdName, osLabel, capturedCmd.name)
	}

	if len(capturedCmd.args) != 1 || capturedCmd.args[0] != url {
		t.Errorf("expected args [%s], got %v", url, capturedCmd.args)
	}
}

//nolint:paralleltest // swaps the package-level execCommand var; cannot run concurrent with same-var tests
func TestOpen_Darwin(t *testing.T) {
	checkOpenSingleArgCommand(t, goosDarwin, "macOS", cmdOpen)
}

//nolint:paralleltest // swaps the package-level execCommand var; cannot run concurrent with same-var tests
func TestOpen_Linux(t *testing.T) {
	checkOpenSingleArgCommand(t, goosLinux, "Linux", cmdXdgOpen)
}

//nolint:paralleltest // swaps the package-level execCommand var; cannot run concurrent with same-var tests
func TestOpen_Windows(t *testing.T) {
	if runtime.GOOS != goosWindows {
		t.Skip("skipping Windows-specific test")
	}

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
		t.Errorf("Open on Windows failed: %v", err)
	}

	if capturedCmd == nil {
		t.Fatal("expected command to be executed")
	}

	if capturedCmd.name != cmdWindowsExe {
		t.Errorf("expected command 'cmd' on Windows, got '%s'", capturedCmd.name)
	}

	expectedArgs := []string{"/c", "start", url}
	if len(capturedCmd.args) != len(expectedArgs) {
		t.Errorf("expected args %v, got %v", expectedArgs, capturedCmd.args)
	} else {
		for i, expected := range expectedArgs {
			if capturedCmd.args[i] != expected {
				t.Errorf("arg[%d]: expected '%s', got '%s'", i, expected, capturedCmd.args[i])
			}
		}
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
