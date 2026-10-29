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
