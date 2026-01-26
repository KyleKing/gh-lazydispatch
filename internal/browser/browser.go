// Package browser provides cross-platform browser opening functionality.
package browser

import (
	"os/exec"
	"runtime"
)

// execCommand is a variable that holds the command executor.
// It can be overridden in tests to avoid actually opening browsers.
var execCommand = func(name string, args ...string) cmdRunner {
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

	switch runtime.GOOS {
	case "darwin":
		name = "open"
		args = []string{url}
	case "linux":
		name = "xdg-open"
		args = []string{url}
	case "windows":
		name = "cmd"
		args = []string{"/c", "start", url}
	default:
		name = "xdg-open"
		args = []string{url}
	}

	cmd := execCommand(name, args...)
	return cmd.Start()
}
