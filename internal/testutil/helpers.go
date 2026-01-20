package testutil

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

// MustMarshalJSON marshals v to JSON, failing the test if an error occurs.
func MustMarshalJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return string(data)
}

// DrainChainUpdates reads all updates from the channel until it closes or times out.
func DrainChainUpdates(t *testing.T, updates <-chan chain.ChainUpdate, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case _, ok := <-updates:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for chain updates")
		}
	}
}

// AssertCommand verifies that an executed command matches expected arguments.
func AssertCommand(t *testing.T, cmd exec.ExecutedCommand, expectedArgs ...string) {
	t.Helper()
	if len(expectedArgs) == 0 {
		return
	}
	if cmd.Name != expectedArgs[0] {
		t.Errorf("command name: got %q, want %q", cmd.Name, expectedArgs[0])
	}
	for i, arg := range expectedArgs[1:] {
		if i >= len(cmd.Args) || cmd.Args[i] != arg {
			found := ""
			if i < len(cmd.Args) {
				found = cmd.Args[i]
			}
			t.Errorf("command arg[%d]: got %q, want %q", i, found, arg)
		}
	}
}

// AssertStepLogNames verifies that step logs have the expected names.
func AssertStepLogNames(t *testing.T, stepLogs []*logs.StepLogs, expectedNames []string) {
	t.Helper()
	if len(stepLogs) != len(expectedNames) {
		t.Errorf("step count: got %d, want %d", len(stepLogs), len(expectedNames))
		return
	}
	for i, name := range expectedNames {
		if stepLogs[i].StepName != name {
			t.Errorf("step[%d] name: got %q, want %q", i, stepLogs[i].StepName, name)
		}
	}
}
