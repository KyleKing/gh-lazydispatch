// Package logs provides workflow run log fetching, caching, filtering, and streaming functionality.
package logs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache stores fetched logs locally for quick access.
type Cache struct {
	cacheDir string
	mu       sync.RWMutex
	entries  map[string]*CacheEntry // keyed by chainName:runID
}

// CacheEntry represents cached log data.
type CacheEntry struct {
	ChainName string
	RunID     int64
	Logs      *RunLogs
	CachedAt  time.Time
	TTL       time.Duration
}

// NewCache creates a new log cache.
// cacheDir should be something like ~/.cache/lazydispatch/logs/
func NewCache(cacheDir string) *Cache {
	return &Cache{
		cacheDir: cacheDir,
		entries:  make(map[string]*CacheEntry),
	}
}

// Get retrieves cached logs if available and not expired.
func (c *Cache) Get(chainName string, runID int64) (*RunLogs, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.makeKey(chainName, runID)

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Since(entry.CachedAt) > entry.TTL {
		return nil, false
	}

	return entry.Logs, true
}

// Put stores logs in the cache.
func (c *Cache) Put(chainName string, runID int64, logs *RunLogs, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.makeKey(chainName, runID)
	entry := &CacheEntry{
		ChainName: chainName,
		RunID:     runID,
		Logs:      logs,
		CachedAt:  time.Now(),
		TTL:       ttl,
	}

	c.entries[key] = entry

	// Persist to disk
	return c.persistEntry(key, entry)
}

// Load loads the cache from disk.
func (c *Cache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache dir: %w", err)
	}

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(c.cacheDir, entry.Name())

		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip invalid entries
		}

		var cacheEntry CacheEntry
		if err := json.Unmarshal(data, &cacheEntry); err != nil {
			continue // Skip invalid entries
		}

		// Check if expired
		if time.Since(cacheEntry.CachedAt) > cacheEntry.TTL {
			os.Remove(path) // Clean up expired entry
			continue
		}

		key := c.makeKey(cacheEntry.ChainName, cacheEntry.RunID)
		c.entries[key] = &cacheEntry
	}

	return nil
}

// Clear removes expired entries from the cache.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.entries {
		if time.Since(entry.CachedAt) > entry.TTL {
			delete(c.entries, key)

			// Remove from disk
			filename := c.makeFilename(key)
			path := filepath.Join(c.cacheDir, filename)
			os.Remove(path)
		}
	}

	return nil
}

// persistEntry writes a cache entry to disk.
func (c *Cache) persistEntry(key string, entry *CacheEntry) error {
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache dir: %w", err)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	filename := c.makeFilename(key)
	path := filepath.Join(c.cacheDir, filename)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// makeKey creates a unique key for a chain run.
func (c *Cache) makeKey(chainName string, runID int64) string {
	return fmt.Sprintf("%s:%d", chainName, runID)
}

// makeFilename creates a filesystem-safe filename from a key.
func (c *Cache) makeFilename(key string) string {
	// Replace colons and slashes for filesystem safety
	safe := key
	safe = filepath.Base(safe)

	return safe + ".json"
}

// Stats returns cache statistics.
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		TotalEntries: len(c.entries),
	}

	for _, entry := range c.entries {
		if time.Since(entry.CachedAt) > entry.TTL {
			stats.ExpiredEntries++
		} else {
			stats.ValidEntries++
		}
	}

	return stats
}

// CacheStats provides cache metrics.
type CacheStats struct {
	TotalEntries   int
	ValidEntries   int
	ExpiredEntries int
}
