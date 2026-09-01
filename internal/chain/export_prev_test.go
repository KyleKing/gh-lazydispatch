package chain_test

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
)

// The preview and the export interpolate the same templates the executor
// resolves at dispatch, so `previous` has to name the step directly above,
// not the one before that.
func TestExportAsBash_PreviousNamesTheStepDirectlyAbove(t *testing.T) {
	t.Parallel()

	def := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "one.yml", Inputs: map[string]string{"tag": "first"}},
			{Workflow: "two.yml", Inputs: map[string]string{"tag": "second"}},
			{Workflow: "three.yml", Inputs: map[string]string{"tag": "{{ previous.inputs.tag }}"}},
		},
	}

	script := chain.ExportAsBash("c", def, nil, "main")

	if !strings.Contains(script, "tag=second") {
		t.Errorf("step three did not take step two's tag:\n%s", script)
	}

	if strings.Contains(script, "tag=first\"") && strings.Count(script, "tag=first") > 1 {
		t.Errorf("step three took step one's tag:\n%s", script)
	}
}
