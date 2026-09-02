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

// openCommand is the platform's own open command and its arguments. It takes
// the OS rather than reading runtime.GOOS so that every platform's branch is
// reachable from the one the tests run on.
func openCommand(goos, url string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{url}
	case "windows":
		return "cmd", []string{"/c", "start", url}
	}

	return "xdg-open", []string{url}
}

// Open opens the specified URL in the default browser.
func Open(url string) error {
	name, args := openCommand(runtime.GOOS, url)

	if err := execCommand(name, args...).Start(); err != nil {
		return fmt.Errorf("opening browser for %s: %w", url, err)
	}

	return nil
}
