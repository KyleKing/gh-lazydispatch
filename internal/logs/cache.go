// Package logs provides workflow run log fetching, caching, filtering, and streaming functionality.
package logs

import (
	"strconv"
	"time"

	"github.com/kyleking/aragonite/cache"
)

// logCacheTTL is how long a fetched log stays current. A completed run's log
// never changes, so the ceiling is there for the run that was still going when
// it was read.
const logCacheTTL = 10 * time.Minute

// newLogCache holds a run's parsed steps in memory only. Aragonite's disk store
// takes counts, states, and titles and never bodies, and a run's log is a body
// measured in megabytes.
func newLogCache() *cache.TTLCache[[]*StepLogs] {
	return cache.NewTTLCache[[]*StepLogs](logCacheTTL)
}

// logCacheKey names a run's logs. The workflow is part of the key because a run
// reached by ID carries no workflow name, and the fetch that names one asks gh
// a different question.
func logCacheKey(runID int64, workflow string) string {
	return strconv.FormatInt(runID, 10) + "\x00" + workflow
}
