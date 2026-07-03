package browser

import (
	"runtime"
	"testing"
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

	url := "https://example.com"
	err := Open(url)
	if err != nil {
		t.Errorf("Open failed: %v", err)
	}

	if capturedCmd == nil {
		t.Fatal("expected command to be executed")
	}

	// Verify correct command based on OS
	switch runtime.GOOS {
	case "darwin":
		if capturedCmd.name != "open" {
			t.Errorf("expected command 'open', got '%s'", capturedCmd.name)
		}
		if len(capturedCmd.args) != 1 || capturedCmd.args[0] != url {
			t.Errorf("expected args [%s], got %v", url, capturedCmd.args)
		}
	case "linux":
		if capturedCmd.name != "xdg-open" {
			t.Errorf("expected command 'xdg-open', got '%s'", capturedCmd.name)
		}
		if len(capturedCmd.args) != 1 || capturedCmd.args[0] != url {
			t.Errorf("expected args [%s], got %v", url, capturedCmd.args)
		}
	case "windows":
		if capturedCmd.name != "cmd" {
			t.Errorf("expected command 'cmd', got '%s'", capturedCmd.name)
		}
		expectedArgs := []string{"/c", "start", url}
		if len(capturedCmd.args) != len(expectedArgs) {
			t.Errorf("expected args %v, got %v", expectedArgs, capturedCmd.args)
		}
	}
}

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

func TestOpen_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("skipping macOS-specific test")
	}

	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	var capturedCmd *mockCmd
	execCommand = func(name string, args ...string) cmdRunner {
		cmd := &mockCmd{name: name, args: args}
		capturedCmd = cmd
		return cmd
	}

	url := "https://example.com"
	err := Open(url)
	if err != nil {
		t.Errorf("Open on macOS failed: %v", err)
	}

	if capturedCmd == nil {
		t.Fatal("expected command to be executed")
	}

	if capturedCmd.name != "open" {
		t.Errorf("expected command 'open' on macOS, got '%s'", capturedCmd.name)
	}

	if len(capturedCmd.args) != 1 || capturedCmd.args[0] != url {
		t.Errorf("expected args [%s], got %v", url, capturedCmd.args)
	}
}

func TestOpen_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping Linux-specific test")
	}

	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	var capturedCmd *mockCmd
	execCommand = func(name string, args ...string) cmdRunner {
		cmd := &mockCmd{name: name, args: args}
		capturedCmd = cmd
		return cmd
	}

	url := "https://example.com"
	err := Open(url)
	if err != nil {
		t.Errorf("Open on Linux failed: %v", err)
	}

	if capturedCmd == nil {
		t.Fatal("expected command to be executed")
	}

	if capturedCmd.name != "xdg-open" {
		t.Errorf("expected command 'xdg-open' on Linux, got '%s'", capturedCmd.name)
	}

	if len(capturedCmd.args) != 1 || capturedCmd.args[0] != url {
		t.Errorf("expected args [%s], got %v", url, capturedCmd.args)
	}
}

func TestOpen_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
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

	url := "https://example.com"
	err := Open(url)
	if err != nil {
		t.Errorf("Open on Windows failed: %v", err)
	}

	if capturedCmd == nil {
		t.Fatal("expected command to be executed")
	}

	if capturedCmd.name != "cmd" {
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
