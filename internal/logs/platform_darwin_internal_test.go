//go:build darwin

package logs

import (
	"testing"
)

func TestDarwin_LogParsing(t *testing.T) {
	t.Parallel()

	// Test log parsing on macOS
	rawLogs := "##[group]Test\nINFO: macOS test\n##[endgroup]"
	entries := ParseLogOutput(rawLogs, "test")

	if len(entries) == 0 {
		t.Error("expected parsed entries")
	}
}
