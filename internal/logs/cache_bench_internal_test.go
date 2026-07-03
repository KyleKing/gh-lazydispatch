package logs

import (
	"testing"
	"time"
)

func BenchmarkCache_Put(b *testing.B) {
	cache := NewCache(b.TempDir())
	runLogs := NewRunLogs("test", "main")

	// Add realistic number of log entries
	for range 100 {
		runLogs.AddStep(&StepLogs{
			Entries: make([]LogEntry, 100),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		cache.Put("test", int64(i), runLogs, 1*time.Hour)
	}
}

func BenchmarkCache_Get(b *testing.B) {
	cache := NewCache(b.TempDir())
	runLogs := NewRunLogs("test", "main")

	// Add realistic data
	for range 100 {
		runLogs.AddStep(&StepLogs{
			Entries: make([]LogEntry, 100),
		})
	}

	cache.Put("test", 123, runLogs, 1*time.Hour)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		cache.Get("test", 123)
	}
}

func BenchmarkCache_ConcurrentAccess(b *testing.B) {
	cache := NewCache(b.TempDir())
	runLogs := NewRunLogs("test", "main")

	// Add realistic data
	for range 50 {
		runLogs.AddStep(&StepLogs{
			Entries: make([]LogEntry, 50),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.Put("test", 123, runLogs, 1*time.Hour)
			cache.Get("test", 123)
		}
	})
}

func BenchmarkCache_Load(b *testing.B) {
	cacheDir := b.TempDir()
	cache1 := NewCache(cacheDir)

	// Setup: Add entries
	for i := range 10 {
		runLogs := NewRunLogs("test", "main")
		for range 50 {
			runLogs.AddStep(&StepLogs{
				Entries: make([]LogEntry, 50),
			})
		}

		cache1.Put("test", int64(i), runLogs, 1*time.Hour)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		cache2 := NewCache(cacheDir)
		cache2.Load()
	}
}

func BenchmarkCache_Stats(b *testing.B) {
	cache := NewCache(b.TempDir())

	// Add entries
	for i := range 100 {
		runLogs := NewRunLogs("test", "main")
		cache.Put("test", int64(i), runLogs, 1*time.Hour)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		cache.Stats()
	}
}

func BenchmarkCache_Clear(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		b.StopTimer()
		cache := NewCache(b.TempDir())

		// Add mix of valid and expired entries
		for j := range 50 {
			runLogs := NewRunLogs("test", "main")

			ttl := 1 * time.Hour
			if j%2 == 0 {
				ttl = 1 * time.Millisecond
			}

			cache.Put("test", int64(j), runLogs, ttl)
		}

		time.Sleep(5 * time.Millisecond) // Wait for some to expire

		b.StartTimer()
		cache.Clear()
	}
}

func BenchmarkCache_MakeKey(b *testing.B) {
	cache := NewCache(b.TempDir())

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		cache.makeKey("test-chain", int64(i))
	}
}

func BenchmarkCache_PutGet_SmallLogs(b *testing.B) {
	cache := NewCache(b.TempDir())
	runLogs := NewRunLogs("test", "main")

	// Small logs: 10 steps with 10 entries each
	for range 10 {
		runLogs.AddStep(&StepLogs{
			Entries: make([]LogEntry, 10),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		cache.Put("test", int64(i%100), runLogs, 1*time.Hour)
		cache.Get("test", int64(i%100))
	}
}

func BenchmarkCache_PutGet_LargeLogs(b *testing.B) {
	cache := NewCache(b.TempDir())
	runLogs := NewRunLogs("test", "main")

	// Large logs: 200 steps with 500 entries each
	for range 200 {
		runLogs.AddStep(&StepLogs{
			Entries: make([]LogEntry, 500),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		cache.Put("test", int64(i%100), runLogs, 1*time.Hour)
		cache.Get("test", int64(i%100))
	}
}
