package runcache

import (
	"path/filepath"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// Jobs maps a finished run's ID to its jobs. A finished run's jobs do not
// change, so an entry never needs refetching.
type Jobs map[int64][]github.JobTiming

// JobsPath returns the job cache file for one repository.
func JobsPath(repo string) string {
	return filepath.Join(Dir(repo), "ci-jobs.json")
}

// LoadJobs reads the job cache at path. A missing file is empty.
func LoadJobs(path string) (Jobs, error) {
	jobs := Jobs{}

	err := readJSON(path, &jobs, "job cache")

	return jobs, err
}

// SaveJobs writes jobs to path through a temporary file.
func SaveJobs(path string, jobs Jobs) error {
	return writeJSON(path, jobs, "job cache")
}

// CacheLogs maps a job ID to what its log said about caches. A finished job's
// log does not change, so an entry never needs refetching.
type CacheLogs map[int64]CacheLog

// CacheLog counts the cache restores a job's log reported.
type CacheLog struct {
	Hits   int `json:"hits"`
	Misses int `json:"misses"`
}

// CacheLogsPath returns the cache-log file for one repository.
func CacheLogsPath(repo string) string {
	return filepath.Join(Dir(repo), "ci-cache-logs.json")
}

// LoadCacheLogs reads the cache-log file at path. A missing file is empty.
func LoadCacheLogs(path string) (CacheLogs, error) {
	logs := CacheLogs{}

	err := readJSON(path, &logs, "cache-log cache")

	return logs, err
}

// SaveCacheLogs writes logs to path through a temporary file.
func SaveCacheLogs(path string, logs CacheLogs) error {
	return writeJSON(path, logs, "cache-log cache")
}
