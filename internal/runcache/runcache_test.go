package runcache_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

func day(n int) time.Time { return time.Date(2026, 1, n, 0, 0, 0, 0, time.UTC) }

func run(id int64, created time.Time, status string) github.RunTiming {
	return github.RunTiming{ID: id, CreatedAt: created, UpdatedAt: created.Add(time.Minute), Status: status}
}

func saveAndLoad(t *testing.T, entry runcache.Entry) runcache.Entry {
	t.Helper()

	path := filepath.Join(t.TempDir(), "nested", "runs.json")
	if err := runcache.Save(path, entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := runcache.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	return loaded
}

// A repeat report fetches only the days the cache does not cover, whether
// they come after it or, for a wider window, before it.
func TestEntry_MissingIsOnlyTheUncoveredRange(t *testing.T) {
	t.Parallel()

	fetched := []github.RunTiming{run(1, day(2), github.StatusCompleted)}
	loaded := saveAndLoad(t, runcache.Entry{}.Merge(fetched, day(1), day(5)))

	whole := runcache.Entry{}.Missing(day(1), day(5))
	if len(whole) != 1 || !whole[0].From.Equal(day(1)) || !whole[0].To.Equal(day(5)) {
		t.Fatalf("an empty cache is missing %v, want the whole range", whole)
	}

	later := loaded.Missing(day(1), day(8))
	if len(later) != 1 || !later[0].From.Equal(day(5)) || !later[0].To.Equal(day(8)) {
		t.Fatalf("missing is %v, want only day 5 onward", later)
	}

	older := loaded.Missing(day(1).AddDate(0, 0, -2), day(5))
	if len(older) != 1 || !older[0].To.Equal(day(1)) {
		t.Fatalf("a wider window is missing %v, want only the days before the cache", older)
	}
}

// A run still in flight is neither stored nor treated as covered, so the next
// report reads it again and it enters the cache once it finishes.
func TestEntry_RetriesInFlightRunsUntilTheyFinish(t *testing.T) {
	t.Parallel()

	fetched := []github.RunTiming{
		run(1, day(1), github.StatusCompleted),
		run(2, day(3), github.StatusCompleted),
		run(3, day(4), "in_progress"),
	}

	loaded := saveAndLoad(t, runcache.Entry{}.Merge(fetched, day(1), day(5)))
	if len(loaded.Runs) != 2 || !loaded.CoveredThrough.Equal(day(4)) {
		t.Fatalf("cache holds %d runs through %v, want 2 through %v", len(loaded.Runs), loaded.CoveredThrough, day(4))
	}

	done := loaded.Merge([]github.RunTiming{run(3, day(4), github.StatusCompleted)}, day(1), day(8))
	if len(done.Runs) != 3 || !done.CoveredThrough.Equal(day(8)) {
		t.Fatalf("finished: %d runs through %v, want 3 through %v", len(done.Runs), done.CoveredThrough, day(8))
	}

	stale := run(5, day(2), "queued")
	kept := runcache.Entry{}.Merge([]github.RunTiming{stale}, day(1), day(8))
	if len(kept.Runs) != 1 || !kept.CoveredThrough.Equal(day(8)) {
		t.Fatalf("a run queued for days holds coverage at %v with %d stored, want %v with 1",
			kept.CoveredThrough, len(kept.Runs), day(8))
	}

	inFlight := []github.RunTiming{run(4, day(7), "queued")}
	if within := runcache.Within(done.Runs, inFlight, day(2), day(8)); len(within) != 3 {
		t.Fatalf("Within returned %d runs, want the 2 stored in range plus the in-flight one", len(within))
	}
}
