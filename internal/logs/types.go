package logs

import (
	"sync"
	"time"
)

// LogEntry represents a single log line with metadata.
type LogEntry struct {
	Timestamp time.Time
	Content   string
	Level     LogLevel // error, warning, info, debug
	StepName  string   // for grouping
}

// LogLevel indicates the severity of a log line.
type LogLevel string

const (
	LogLevelError   LogLevel = "error"
	LogLevelWarning LogLevel = "warning"
	LogLevelInfo    LogLevel = "info"
	LogLevelDebug   LogLevel = "debug"
	LogLevelUnknown LogLevel = "unknown"
)

// StepLogs contains all log entries for a single workflow step.
type StepLogs struct {
	StepIndex  int
	Workflow   string
	RunID      int64
	JobName    string
	StepName   string
	Status     string
	Conclusion string
	Entries    []LogEntry
	FetchedAt  time.Time
	Error      error
}

// RunLogs contains logs for all steps in a workflow run or chain.
type RunLogs struct {
	ChainName string
	Branch    string
	Steps     []*StepLogs
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
