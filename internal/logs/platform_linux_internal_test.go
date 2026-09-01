//go:build linux

package logs

import (
	"testing"
)

func TestLinux_LogParsing(t *testing.T) {
	t.Parallel()

	// Test log parsing on Linux
	rawLogs := "##[group]Test\nINFO: Linux test\n##[endgroup]"
	entries := ParseLogOutput(rawLogs, "test")

	if len(entries) == 0 {
		t.Error("expected parsed entries")
	}
}
