package logs

import (
	"sync"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// LogEntry represents a single log line with metadata.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
	Level     LogLevel  `json:"level"`     // error, warning, info, debug
	StepName  string    `json:"step_name"` // for grouping
}

// LogLevel indicates the severity of a log line.
type LogLevel string

// Log severity levels.
const (
	LogLevelError   LogLevel = "error"
	LogLevelWarning LogLevel = "warning"
	LogLevelInfo    LogLevel = "info"
	LogLevelDebug   LogLevel = "debug"
	LogLevelUnknown LogLevel = "unknown"
)

// StepLogs contains all log entries for a single workflow step.
type StepLogs struct {
	FetchedAt  time.Time `json:"fetched_at"`
	Error      error     `json:"-"`
	Workflow   string    `json:"workflow"`
	JobName    string    `json:"job_name"`
	StepName   string    `json:"step_name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	// JobStatus and JobConclusion are the owning job's, which a step with a
	// blank conclusion cannot be judged without.
	JobStatus     string     `json:"job_status"`
	JobConclusion string     `json:"job_conclusion"`
	Entries       []LogEntry `json:"entries"`
	StepIndex     int        `json:"step_index"`
	RunID         int64      `json:"run_id"`
	JobID         int64      `json:"job_id"`
}

// Unsuccessful reports whether the step ended as anything but a success or a
// skip. A blank conclusion counts once its job has completed, because GitHub
// leaves it blank on a step the runner died inside.
func (s *StepLogs) Unsuccessful() bool {
	if s.Conclusion == "" {
		return s.JobStatus == github.StatusCompleted
	}

	return s.Conclusion != github.ConclusionSuccess && s.Conclusion != github.ConclusionSkipped
}

// JobFailed reports whether the step's job completed as anything but a success
// or a skip.
func (s *StepLogs) JobFailed() bool {
	return s.JobStatus == github.StatusCompleted &&
		s.JobConclusion != github.ConclusionSuccess &&
		s.JobConclusion != github.ConclusionSkipped
}

// Failed reports whether the step is one a diagnosis should read. A step whose
// conclusion is empty has not reported one yet, which is not a failure.
func (s *StepLogs) Failed() bool {
	return s.Conclusion != "" && s.Conclusion != "success" && s.Conclusion != "skipped"
}

// RunLogs contains logs for all steps in a workflow run or chain.
type RunLogs struct {
	ChainName string      `json:"chain_name"`
	Branch    string      `json:"branch"`
	Steps     []*StepLogs `json:"steps"`
	mu        sync.RWMutex
}

// NewRunLogs creates a new RunLogs instance.
func NewRunLogs(chainName, branch string) *RunLogs {
	return &RunLogs{
		ChainName: chainName,
		Branch:    branch,
		Steps:     make([]*StepLogs, 0),
	}
}

// AddStep adds step logs to the run logs.
func (rl *RunLogs) AddStep(stepLogs *StepLogs) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.Steps = append(rl.Steps, stepLogs)
}

// GetStep returns step logs by index.
func (rl *RunLogs) GetStep(idx int) *StepLogs {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if idx >= 0 && idx < len(rl.Steps) {
		return rl.Steps[idx]
	}

	return nil
}

// AllSteps returns all step logs.
func (rl *RunLogs) AllSteps() []*StepLogs {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	steps := make([]*StepLogs, len(rl.Steps))
	copy(steps, rl.Steps)

	return steps
}

// FilterLevel represents different log filtering modes.
type FilterLevel string

// Log filtering modes.
const (
	FilterAll      FilterLevel = "all"
	FilterErrors   FilterLevel = "errors"
	FilterWarnings FilterLevel = "warnings"
	FilterCustom   FilterLevel = "custom"
)

// FilterConfig configures log filtering.
type FilterConfig struct {
	Level         FilterLevel
	SearchTerm    string
	CaseSensitive bool
	Regex         bool
	StepIndex     int // -1 for all steps
}

// NewFilterConfig creates a default filter config.
func NewFilterConfig() *FilterConfig {
	return &FilterConfig{
		Level:         FilterAll,
		SearchTerm:    "",
		CaseSensitive: false,
		Regex:         false,
		StepIndex:     -1,
	}
}
