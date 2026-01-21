package logs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCache_NewCache(t *testing.T) {
	cacheDir := t.TempDir()
	cache := NewCache(cacheDir)

	if cache == nil {
		t.Fatal("expected non-nil cache")
	}

	if cache.cacheDir != cacheDir {
		t.Errorf("cacheDir: got %q, want %q", cache.cacheDir, cacheDir)
	}

	if cache.entries == nil {
		t.Error("expected non-nil entries map")
	}
}

func TestCache_GetPut(t *testing.T) {
	cache := NewCache(t.TempDir())
	runLogs := NewRunLogs("test", "main")
	runLogs.AddStep(&StepLogs{StepName: "build"})

	// Put logs into cache
	err := cache.Put("test", 123, runLogs, 1*time.Hour)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Get logs from cache
	retrieved, found := cache.Get("test", 123)
	if !found {
		t.Fatal("expected to find cached logs")
	}

	if retrieved == nil {
		t.Fatal("expected non-nil logs")
	}

	if retrieved.ChainName != "test" {
		t.Errorf("ChainName: got %q, want %q", retrieved.ChainName, "test")
	}

	if len(retrieved.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(retrieved.Steps))
	}
}

func TestCache_Expiration(t *testing.T) {
	cache := NewCache(t.TempDir())
	runLogs := NewRunLogs("test", "main")

	// Put with very short TTL
	err := cache.Put("test", 123, runLogs, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Try to get expired entry
	_, found := cache.Get("test", 123)
	if found {
		t.Error("expected expired entry not found")
	}
}

func TestCache_Load(t *testing.T) {
	cacheDir := t.TempDir()
	cache1 := NewCache(cacheDir)

	// Add entries to first cache instance
	runLogs1 := NewRunLogs("test1", "main")
	runLogs2 := NewRunLogs("test2", "develop")

	err := cache1.Put("test1", 123, runLogs1, 1*time.Hour)
	if err != nil {
		t.Fatalf("Put test1 failed: %v", err)
	}

	err = cache1.Put("test2", 456, runLogs2, 1*time.Hour)
	if err != nil {
		t.Fatalf("Put test2 failed: %v", err)
	}

	// Create new cache instance and load from disk
	cache2 := NewCache(cacheDir)
	err = cache2.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify entries were loaded
	retrieved1, found := cache2.Get("test1", 123)
	if !found {
		t.Error("expected to find test1 after load")
	}
	if retrieved1.ChainName != "test1" {
		t.Errorf("test1 ChainName: got %q, want %q", retrieved1.ChainName, "test1")
	}

	retrieved2, found := cache2.Get("test2", 456)
	if !found {
		t.Error("expected to find test2 after load")
	}
	if retrieved2.ChainName != "test2" {
		t.Errorf("test2 ChainName: got %q, want %q", retrieved2.ChainName, "test2")
	}
}

func TestCache_LoadWithExpiredEntries(t *testing.T) {
	cacheDir := t.TempDir()
	cache1 := NewCache(cacheDir)

	// Add entry with short TTL
	runLogs := NewRunLogs("test", "main")
	err := cache1.Put("test", 123, runLogs, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Load into new cache - expired entries should be cleaned up
	cache2 := NewCache(cacheDir)
	err = cache2.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Expired entry should not be loaded
	_, found := cache2.Get("test", 123)
	if found {
		t.Error("expected expired entry not loaded")
	}

	// Verify file was removed
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 cache files, got %d", len(files))
	}
}

func TestCache_Clear(t *testing.T) {
	cacheDir := t.TempDir()
	cache := NewCache(cacheDir)

	// Add valid and expired entries
	runLogs1 := NewRunLogs("valid", "main")
	runLogs2 := NewRunLogs("expired", "main")

	err := cache.Put("valid", 123, runLogs1, 1*time.Hour)
	if err != nil {
		t.Fatalf("Put valid failed: %v", err)
	}

	err = cache.Put("expired", 456, runLogs2, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Put expired failed: %v", err)
	}

	// Wait for one to expire
	time.Sleep(5 * time.Millisecond)

	// Clear expired entries
	err = cache.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Valid entry should still exist
	_, found := cache.Get("valid", 123)
	if !found {
		t.Error("expected valid entry still exists")
	}

	// Expired entry should be removed
	_, found = cache.Get("expired", 456)
	if found {
		t.Error("expected expired entry removed")
	}

	// Verify file was removed
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 cache file, got %d", len(files))
	}
}

func TestCache_Stats(t *testing.T) {
	cache := NewCache(t.TempDir())

	// Initially empty
	stats := cache.Stats()
	if stats.TotalEntries != 0 {
		t.Errorf("TotalEntries: got %d, want 0", stats.TotalEntries)
	}

	// Add valid entries
	runLogs1 := NewRunLogs("test1", "main")
	runLogs2 := NewRunLogs("test2", "main")
	cache.Put("test1", 123, runLogs1, 1*time.Hour)
	cache.Put("test2", 456, runLogs2, 1*time.Hour)

	stats = cache.Stats()
	if stats.TotalEntries != 2 {
		t.Errorf("TotalEntries: got %d, want 2", stats.TotalEntries)
	}
	if stats.ValidEntries != 2 {
		t.Errorf("ValidEntries: got %d, want 2", stats.ValidEntries)
	}
	if stats.ExpiredEntries != 0 {
		t.Errorf("ExpiredEntries: got %d, want 0", stats.ExpiredEntries)
	}

	// Add expired entry
	runLogs3 := NewRunLogs("test3", "main")
	cache.Put("test3", 789, runLogs3, 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	stats = cache.Stats()
	if stats.TotalEntries != 3 {
		t.Errorf("TotalEntries: got %d, want 3", stats.TotalEntries)
	}
	if stats.ValidEntries != 2 {
		t.Errorf("ValidEntries: got %d, want 2", stats.ValidEntries)
	}
	if stats.ExpiredEntries != 1 {
		t.Errorf("ExpiredEntries: got %d, want 1", stats.ExpiredEntries)
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	cache := NewCache(t.TempDir())

	const numGoroutines = 10
	const opsPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				runLogs := NewRunLogs("test", "main")
				cache.Put("test", int64(id*opsPerGoroutine+j), runLogs, 1*time.Hour)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				cache.Get("test", int64(id*opsPerGoroutine+j))
			}
		}(i)
	}

	wg.Wait()

	// Verify entries were added
	stats := cache.Stats()
	expectedTotal := numGoroutines * opsPerGoroutine
	if stats.TotalEntries != expectedTotal {
		t.Errorf("TotalEntries: got %d, want %d", stats.TotalEntries, expectedTotal)
	}
}

func TestCache_InvalidJSON(t *testing.T) {
	cacheDir := t.TempDir()

	// Write invalid JSON to cache file
	invalidJSON := []byte("{invalid json}")
	filename := filepath.Join(cacheDir, "test_123.json")
	err := os.WriteFile(filename, invalidJSON, 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Load should skip invalid entries
	cache := NewCache(cacheDir)
	err = cache.Load()
	if err != nil {
		t.Fatalf("Load should not fail on invalid JSON: %v", err)
	}

	// Verify no entries loaded
	stats := cache.Stats()
	if stats.TotalEntries != 0 {
		t.Errorf("expected 0 entries, got %d", stats.TotalEntries)
	}
}

func TestCache_MakeKey(t *testing.T) {
	cache := NewCache(t.TempDir())

	tests := []struct {
		name      string
		chainName string
		runID     int64
		want      string
	}{
		{"simple", "test", 123, "test:123"},
		{"with hyphen", "my-chain", 456, "my-chain:456"},
		{"large run ID", "test", 9876543210, "test:9876543210"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cache.makeKey(tt.chainName, tt.runID)
			if got != tt.want {
				t.Errorf("makeKey: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCache_GetNonExistent(t *testing.T) {
	cache := NewCache(t.TempDir())

	// Try to get non-existent entry
	_, found := cache.Get("nonexistent", 999)
	if found {
		t.Error("expected not found for non-existent entry")
	}
}

func TestCache_PersistEntry(t *testing.T) {
	cacheDir := t.TempDir()
	cache := NewCache(cacheDir)

	runLogs := NewRunLogs("test", "main")
	runLogs.AddStep(&StepLogs{StepName: "build"})

	err := cache.Put("test", 123, runLogs, 1*time.Hour)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify file was created
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 cache file, got %d", len(files))
	}

	// Verify file contents
	filename := files[0].Name()
	data, err := os.ReadFile(filepath.Join(cacheDir, filename))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var entry CacheEntry
	err = json.Unmarshal(data, &entry)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if entry.ChainName != "test" {
		t.Errorf("ChainName: got %q, want %q", entry.ChainName, "test")
	}

	if entry.RunID != 123 {
		t.Errorf("RunID: got %d, want 123", entry.RunID)
	}
}

func TestCacheEntry_Fields(t *testing.T) {
	now := time.Now()
	runLogs := NewRunLogs("test", "main")

	entry := &CacheEntry{
		ChainName: "test-chain",
		RunID:     12345,
		Logs:      runLogs,
		CachedAt:  now,
		TTL:       1 * time.Hour,
	}

	if entry.ChainName != "test-chain" {
		t.Errorf("ChainName: got %q, want %q", entry.ChainName, "test-chain")
	}

	if entry.RunID != 12345 {
		t.Errorf("RunID: got %d, want 12345", entry.RunID)
	}

	if entry.Logs != runLogs {
		t.Error("Logs mismatch")
	}

	if entry.CachedAt != now {
		t.Error("CachedAt mismatch")
	}

	if entry.TTL != 1*time.Hour {
		t.Errorf("TTL: got %v, want %v", entry.TTL, 1*time.Hour)
	}
}
