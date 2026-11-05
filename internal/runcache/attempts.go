package runcache

import (
	"path/filepath"
	"strconv"
)

// AttemptJob is what a failure classification needs from one job of one run
// attempt, so the cache holds no step timestamps.
type AttemptJob struct {
	Name         string `json:"name"`
	Conclusion   string `json:"conclusion"`
	StepFailed   bool   `json:"step_failed,omitempty"`
	StepTimedOut bool   `json:"step_timed_out,omitempty"`
}

// Attempts maps a finished run attempt (see AttemptKey) to its jobs. A finished
// attempt's jobs do not change, so an entry never needs refetching.
type Attempts map[string][]AttemptJob

// AttemptKey names one attempt of one run in Attempts.
func AttemptKey(runID int64, attempt int) string {
	return strconv.FormatInt(runID, 10) + "/" + strconv.Itoa(attempt)
}

// AttemptsPath returns the attempt cache file for one repository.
func AttemptsPath(repo string) string {
	return filepath.Join(Dir(repo), "ci-attempts.json")
}

// LoadAttempts reads the attempt cache at path. A missing file is empty.
func LoadAttempts(path string) (Attempts, error) {
	attempts := Attempts{}

	err := readJSON(path, &attempts, "attempt cache")

	return attempts, err
}

// SaveAttempts writes attempts to path through a temporary file.
func SaveAttempts(path string, attempts Attempts) error {
	return writeJSON(path, attempts, "attempt cache")
}
