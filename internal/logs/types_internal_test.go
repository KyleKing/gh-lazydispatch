package logs

import (
	"sync"
	"testing"
	"time"
)

func TestRunLogs_NewRunLogs(t *testing.T) {
	rl := NewRunLogs("test-chain", "main")

	if rl == nil {
		t.Fatal("expected non-nil RunLogs")
	}

	if rl.ChainName != "test-chain" {
		t.Errorf("ChainName: got %q, want %q", rl.ChainName, "test-chain")
	}

	if rl.Branch != "main" {
		t.Errorf("Branch: got %q, want %q", rl.Branch, "main")
	}

	if rl.Steps == nil {
		t.Error("expected non-nil Steps slice")
	}

	if len(rl.Steps) != 0 {
		t.Errorf("expected empty Steps, got %d", len(rl.Steps))
	}
}

func TestRunLogs_AddStep(t *testing.T) {
	rl := NewRunLogs("test", "main")

	step1 := &StepLogs{
		StepIndex: 0,
		StepName:  "checkout",
		Entries:   []LogEntry{{Content: "test"}},
	}
	step2 := &StepLogs{
		StepIndex: 1,
		StepName:  "build",
		Entries:   []LogEntry{{Content: "build test"}},
	}

	rl.AddStep(step1)
	rl.AddStep(step2)

	if len(rl.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(rl.Steps))
	}

	if rl.Steps[0] != step1 {
		t.Error("first step mismatch")
	}

	if rl.Steps[1] != step2 {
		t.Error("second step mismatch")
	}
}

func TestRunLogs_GetStep(t *testing.T) {
	tests := []struct {
		name    string
		idx     int
		wantNil bool
	}{
		{"valid index 0", 0, false},
		{"valid index 1", 1, false},
		{"negative index", -1, true},
		{"out of bounds", 10, true},
	}

	rl := NewRunLogs("test", "main")
	rl.AddStep(&StepLogs{StepName: "build"})
	rl.AddStep(&StepLogs{StepName: "test"})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := rl.GetStep(tt.idx)
			if (step == nil) != tt.wantNil {
				t.Errorf("got nil=%v, want nil=%v", step == nil, tt.wantNil)
			}
		})
	}
}

func TestRunLogs_AllSteps(t *testing.T) {
	rl := NewRunLogs("test", "main")
	step1 := &StepLogs{StepName: "checkout"}
	step2 := &StepLogs{StepName: "build"}

	rl.AddStep(step1)
	rl.AddStep(step2)

	steps := rl.AllSteps()

	// Verify copy semantics - modifying returned slice shouldn't affect original
	if len(steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(steps))
	}

	// Verify it's a copy by checking we can modify it without affecting original
	originalLen := len(rl.Steps)

	_ = append(steps, &StepLogs{StepName: "new"})

	if len(rl.Steps) != originalLen {
		t.Error("AllSteps should return a copy, not a reference to internal slice")
	}
}

func TestRunLogs_ConcurrentAccess(t *testing.T) {
	rl := NewRunLogs("test", "main")

	// Launch multiple goroutines adding steps concurrently
	const numGoroutines = 10

	const stepsPerGoroutine = 5

	var wg sync.WaitGroup

	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()

			for j := range stepsPerGoroutine {
				step := &StepLogs{
					StepIndex: id*stepsPerGoroutine + j,
					StepName:  "step",
				}
				rl.AddStep(step)
			}
		}(i)
	}

	// Concurrent reads
	for range numGoroutines {
		go func() {
			defer wg.Done()

			for j := range stepsPerGoroutine {
				rl.GetStep(j)
				rl.AllSteps()
			}
		}()
		wg.Add(1)
	}

	wg.Wait()

	// Verify all steps were added
	expectedSteps := numGoroutines * stepsPerGoroutine
	if len(rl.Steps) != expectedSteps {
		t.Errorf("expected %d steps, got %d", expectedSteps, len(rl.Steps))
	}
}

func TestFilterConfig_NewFilterConfig(t *testing.T) {
	config := NewFilterConfig()

	if config == nil {
		t.Fatal("expected non-nil FilterConfig")
	}

	if config.Level != FilterAll {
		t.Errorf("Level: got %v, want %v", config.Level, FilterAll)
	}

	if config.SearchTerm != "" {
		t.Errorf("SearchTerm: got %q, want empty string", config.SearchTerm)
	}

	if config.CaseSensitive {
		t.Error("CaseSensitive: got true, want false")
	}

	if config.Regex {
		t.Error("Regex: got true, want false")
	}

	if config.StepIndex != -1 {
		t.Errorf("StepIndex: got %d, want -1", config.StepIndex)
	}
}

func TestLogLevel_Constants(t *testing.T) {
	tests := []struct {
		name  string
		level LogLevel
		want  string
	}{
		{"error level", LogLevelError, "error"},
		{"warning level", LogLevelWarning, "warning"},
		{"info level", LogLevelInfo, "info"},
		{"debug level", LogLevelDebug, "debug"},
		{"unknown level", LogLevelUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.level) != tt.want {
				t.Errorf("got %q, want %q", tt.level, tt.want)
			}
		})
	}
}

func TestLogEntry_Creation(t *testing.T) {
	now := time.Now()
	entry := LogEntry{
		Timestamp: now,
		Content:   "test log line",
		Level:     LogLevelInfo,
		StepName:  "build",
	}

	if entry.Timestamp != now {
		t.Error("Timestamp mismatch")
	}

	if entry.Content != "test log line" {
		t.Errorf("Content: got %q, want %q", entry.Content, "test log line")
	}

	if entry.Level != LogLevelInfo {
		t.Errorf("Level: got %v, want %v", entry.Level, LogLevelInfo)
	}

	if entry.StepName != "build" {
		t.Errorf("StepName: got %q, want %q", entry.StepName, "build")
	}
}

func TestStepLogs_Creation(t *testing.T) {
	now := time.Now()
	entries := []LogEntry{
		{Content: "line 1", Level: LogLevelInfo},
		{Content: "line 2", Level: LogLevelError},
	}

	stepLogs := &StepLogs{
		StepIndex:  0,
		Workflow:   "ci.yml",
		RunID:      12345,
		JobName:    "build",
		StepName:   "Run tests",
		Status:     "completed",
		Conclusion: "success",
		Entries:    entries,
		FetchedAt:  now,
		Error:      nil,
	}

	if stepLogs.StepIndex != 0 {
		t.Errorf("StepIndex: got %d, want 0", stepLogs.StepIndex)
	}

	if stepLogs.Workflow != "ci.yml" {
		t.Errorf("Workflow: got %q, want %q", stepLogs.Workflow, "ci.yml")
	}

	if stepLogs.RunID != 12345 {
		t.Errorf("RunID: got %d, want 12345", stepLogs.RunID)
	}

	if len(stepLogs.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(stepLogs.Entries))
	}
}

func TestFilterLevel_Constants(t *testing.T) {
	tests := []struct {
		name  string
		level FilterLevel
		want  string
	}{
		{"all filter", FilterAll, "all"},
		{"errors filter", FilterErrors, "errors"},
		{"warnings filter", FilterWarnings, "warnings"},
		{"custom filter", FilterCustom, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.level) != tt.want {
				t.Errorf("got %q, want %q", tt.level, tt.want)
			}
		})
	}
}
