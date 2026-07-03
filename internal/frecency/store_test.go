package frecency_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
)

func TestStore_Record(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	store.Record("owner/repo", "deploy.yml", "main", map[string]string{"env": "prod"})

	entries := store.Entries["owner/repo"]
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Workflow != "deploy.yml" {
		t.Errorf("expected workflow 'deploy.yml', got %q", e.Workflow)
	}

	if e.Branch != "main" {
		t.Errorf("expected branch 'main', got %q", e.Branch)
	}

	if e.RunCount != 1 {
		t.Errorf("expected run count 1, got %d", e.RunCount)
	}
}

func TestStore_Record_Increment(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	store.Record("owner/repo", "deploy.yml", "main", map[string]string{"env": "prod"})
	store.Record("owner/repo", "deploy.yml", "main", map[string]string{"env": "prod"})
	store.Record("owner/repo", "deploy.yml", "main", map[string]string{"env": "prod"})

	entries := store.Entries["owner/repo"]
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (incremented), got %d", len(entries))
	}

	if entries[0].RunCount != 3 {
		t.Errorf("expected run count 3, got %d", entries[0].RunCount)
	}
}

func TestStore_Record_DifferentInputs(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	store.Record("owner/repo", "deploy.yml", "main", map[string]string{"env": "prod"})
	store.Record("owner/repo", "deploy.yml", "main", map[string]string{"env": "staging"})

	entries := store.Entries["owner/repo"]
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (different inputs), got %d", len(entries))
	}
}

func TestStore_TopForRepo(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	store.Record("owner/repo", "deploy.yml", "main", nil)
	store.Record("owner/repo", "ci.yml", "main", nil)
	store.Record("owner/repo", "deploy.yml", "main", nil)
	store.Record("owner/repo", "deploy.yml", "main", nil)

	top := store.TopForRepo("owner/repo", "", 10)
	if len(top) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(top))
	}

	if top[0].Workflow != "deploy.yml" {
		t.Errorf("expected deploy.yml first (higher frecency), got %q", top[0].Workflow)
	}
}

func TestStore_TopForRepo_FilterByWorkflow(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	store.Record("owner/repo", "deploy.yml", "main", nil)
	store.Record("owner/repo", "ci.yml", "main", nil)

	top := store.TopForRepo("owner/repo", "deploy.yml", 10)
	if len(top) != 1 {
		t.Fatalf("expected 1 entry (filtered), got %d", len(top))
	}

	if top[0].Workflow != "deploy.yml" {
		t.Errorf("expected deploy.yml, got %q", top[0].Workflow)
	}
}

func TestStore_TopForRepo_Limit(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	for i := range 10 {
		store.Record("owner/repo", "deploy.yml", "main", map[string]string{"i": string(rune('0' + i))})
	}

	top := store.TopForRepo("owner/repo", "", 5)
	if len(top) != 5 {
		t.Errorf("expected 5 entries (limited), got %d", len(top))
	}
}

func TestStore_SaveLoad(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "history.json")

	store := frecency.NewStore()
	store.Record("owner/repo", "deploy.yml", "main", map[string]string{"env": "prod"})

	if err := store.SaveTo(path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	loaded, err := frecency.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	entries := loaded.Entries["owner/repo"]
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after load, got %d", len(entries))
	}

	if entries[0].Workflow != "deploy.yml" {
		t.Errorf("expected workflow 'deploy.yml', got %q", entries[0].Workflow)
	}
}

func TestLoadFrom_NotFound(t *testing.T) {
	t.Parallel()

	store, err := frecency.LoadFrom("/nonexistent/path/history.json")
	if err != nil {
		t.Fatalf("LoadFrom should not error on missing file: %v", err)
	}

	if store == nil {
		t.Fatal("expected non-nil store")
	}

	if len(store.Entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(store.Entries))
	}
}

func TestScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entry   frecency.HistoryEntry
		wantMin float64
		wantMax float64
	}{
		{
			name: "recent high frequency",
			entry: frecency.HistoryEntry{
				RunCount:  10,
				LastRunAt: time.Now().Add(-30 * time.Minute),
			},
			wantMin: 35.0,
			wantMax: 45.0,
		},
		{
			name: "old low frequency",
			entry: frecency.HistoryEntry{
				RunCount:  1,
				LastRunAt: time.Now().Add(-30 * 24 * time.Hour),
			},
			wantMin: 0.4,
			wantMax: 0.6,
		},
		{
			name: "today medium frequency",
			entry: frecency.HistoryEntry{
				RunCount:  5,
				LastRunAt: time.Now().Add(-6 * time.Hour),
			},
			wantMin: 9.0,
			wantMax: 11.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			score := frecency.Score(tt.entry)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("frecency.Score() = %v, want between %v and %v", score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestSortByFrecency(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []frecency.HistoryEntry{
		{Workflow: "low", RunCount: 1, LastRunAt: now.Add(-30 * 24 * time.Hour)},
		{Workflow: "high", RunCount: 10, LastRunAt: now.Add(-1 * time.Hour)},
		{Workflow: "medium", RunCount: 5, LastRunAt: now.Add(-6 * time.Hour)},
	}

	frecency.SortByFrecency(entries)

	if entries[0].Workflow != "high" {
		t.Errorf("expected 'high' first, got %q", entries[0].Workflow)
	}

	if entries[1].Workflow != "medium" {
		t.Errorf("expected 'medium' second, got %q", entries[1].Workflow)
	}

	if entries[2].Workflow != "low" {
		t.Errorf("expected 'low' third, got %q", entries[2].Workflow)
	}
}
