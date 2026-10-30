package telemetry_test

import (
	"path/filepath"
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/telemetry"
)

// Counts survive a restart, the first-seen time stays put while last-seen
// moves, and a nil tracker is a no-op.
func TestTracker_PersistsCountsAcrossOpensAndNilTracksNothing(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "telemetry.json")

	first, err := telemetry.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	first.Track("command:runs")
	first.Track("command:runs")
	first.Track("dispatch")

	second, err := telemetry.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	second.Track("command:runs")

	store, err := telemetry.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	runs := store.Events["command:runs"]
	if runs.Count != 3 || store.Events["dispatch"].Count != 1 || runs.LastSeen.Before(runs.FirstSeen) {
		t.Errorf("events = %+v, want 3 runs and 1 dispatch", store.Events)
	}

	var none *telemetry.Tracker
	none.Track("ignored")
}

func TestLoad_ReturnsAnEmptyStoreForAMissingFile(t *testing.T) {
	t.Parallel()

	store, err := telemetry.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil || len(store.Events) != 0 {
		t.Errorf("Load = %+v, %v; want an empty store", store, err)
	}
}
