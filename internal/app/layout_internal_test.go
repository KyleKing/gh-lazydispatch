package app

import "testing"

// The config pane's own content decides its height, so a workflow with no
// inputs must not spend half the terminal on an empty table.
func TestLayoutFor_GivesTheConfigPaneWhatItsContentNeeds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		height     int
		wantConfig int
		config     int
		top        int
	}{
		{name: "no workflow selected", height: 24, wantConfig: configPaneEmptyHeight, config: 5, top: 17},
		{name: "one input", height: 24, wantConfig: configPaneChrome + 1, config: 11, top: 11},
		{
			name:   "twenty inputs cannot starve the lists",
			height: 24, wantConfig: configPaneChrome + 20, config: 15, top: 7,
		},
		{
			name:   "a tall terminal gives the rest to the lists",
			height: 50, wantConfig: configPaneChrome + 3, config: 13, top: 35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			box := layoutFor(120, tt.height, tt.wantConfig)

			if box.configHeight != tt.config || box.topHeight != tt.top {
				t.Errorf("config/top are %d/%d, want %d/%d", box.configHeight, box.topHeight, tt.config, tt.top)
			}

			if got := box.topHeight + box.configHeight + viewsFixedChromeHeight; got != tt.height {
				t.Errorf("the split covers %d rows, want %d", got, tt.height)
			}

			if box.leftWidth+box.rightWidth != 120 {
				t.Errorf("the split covers %d columns, want 120", box.leftWidth+box.rightWidth)
			}
		})
	}
}
