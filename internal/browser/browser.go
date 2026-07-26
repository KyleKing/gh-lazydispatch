// Package browser provides cross-platform browser opening functionality.
package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// execCommand is a variable that holds the command executor.
// It can be overridden in tests to avoid actually opening browsers.
var execCommand = func(name string, args ...string) cmdRunner {
	// #nosec G204 -- launches the platform's fixed open command with the URL argument
	return exec.Command(name, args...)
}

// cmdRunner is an interface for command execution.
type cmdRunner interface {
	Start() error
}

// Open opens the specified URL in the default browser.
func Open(url string) error {
	var name string
	var args []string

	const windowsOpenVerb = "start"

	switch runtime.GOOS {
	case "darwin":
		name = "open"
		args = []string{url}
	case "linux":
		name = "xdg-open"
		args = []string{url}
	case "windows":
		name = "cmd"
		args = []string{"/c", windowsOpenVerb, url}
	default:
		name = "xdg-open"
		args = []string{url}
	}

	cmd := execCommand(name, args...)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening browser for %s: %w", url, err)
	}

	return nil
}
