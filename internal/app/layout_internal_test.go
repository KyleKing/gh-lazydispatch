package app

import "testing"

// The left column is sized from the bottom up: the config pane takes what its
// content needs, the chains pane takes its rows or vanishes, and the workflow
// list keeps the rest. When they do not all fit, chains gives ground first.
func TestLayoutFor_SizesTheLeftColumnFromTheBottomUp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		height     int
		wantConfig int
		wantChains int
		config     int
		chains     int
		workflows  int
	}{
		{
			name: "no workflow and no chains", height: 24,
			wantConfig: configPaneEmptyHeight, config: 5, workflows: 17,
		},
		{
			name: "one input beside two chains", height: 30,
			wantConfig: configPaneChrome + 1, wantChains: chainsPaneChrome + 2,
			config: 11, chains: 6, workflows: 11,
		},
		{
			name: "chains give ground before the config pane", height: 24,
			wantConfig: configPaneChrome + 8, wantChains: chainsPaneChrome + 6,
			config: 16, chains: 0, workflows: 6,
		},
		{
			name: "a tall terminal gives the rest to the workflow list", height: 50,
			wantConfig: configPaneChrome + 3, wantChains: chainsPaneChrome + 2,
			config: 13, chains: 6, workflows: 29,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			box := layoutFor(120, tt.height, tt.wantConfig, tt.wantChains)

			if box.configHeight != tt.config || box.chainsHeight != tt.chains || box.workflowHeight != tt.workflows {
				t.Errorf("config/chains/workflows are %d/%d/%d, want %d/%d/%d",
					box.configHeight, box.chainsHeight, box.workflowHeight, tt.config, tt.chains, tt.workflows)
			}

			stacked := box.workflowHeight + box.chainsHeight + box.configHeight
			if stacked != box.rightHeight {
				t.Errorf("the left column is %d rows against a right panel of %d", stacked, box.rightHeight)
			}

			if got := box.rightHeight + viewsFixedChromeHeight; got != tt.height {
				t.Errorf("the split covers %d rows, want %d", got, tt.height)
			}

			if box.leftWidth+box.rightWidth != 120 {
				t.Errorf("the split covers %d columns, want 120", box.leftWidth+box.rightWidth)
			}
		})
	}
}
