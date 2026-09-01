package logs

import (
	"strings"
	"testing"
	"time"
)

func exportFixture() *RunLogs {
	runLogs := NewRunLogs("deploy", "main")
	runLogs.AddStep(&StepLogs{
		StepIndex:  0,
		Workflow:   "build.yml",
		Status:     "completed",
		Conclusion: "success",
		Entries: []LogEntry{
			{Content: "compiling", Level: LogLevelInfo, Timestamp: time.Unix(0, 0)},
		},
	})
	runLogs.AddStep(&StepLogs{
		StepIndex:  1,
		Workflow:   "deploy.yml",
		Status:     "completed",
		Conclusion: "failure",
		Entries: []LogEntry{
			{Content: "connecting", Level: LogLevelInfo, Timestamp: time.Unix(0, 0)},
			{Content: "Error: permission denied to push", Level: LogLevelError, Timestamp: time.Unix(0, 0)},
			{Content: "Error: permission denied again", Level: LogLevelError, Timestamp: time.Unix(0, 0)},
		},
	})

	return runLogs
}

func TestExportAsMarkdown(t *testing.T) {
	t.Parallel()

	errorsOnly := NewFilterConfig()
	errorsOnly.Level = FilterErrors

	noMatch := NewFilterConfig()
	noMatch.SearchTerm = "nothing here matches"

	tests := []struct {
		config   *FilterConfig
		name     string
		runLogs  *RunLogs
		contains []string
		absent   []string
	}{
		{
			name:    "every step and its status",
			runLogs: exportFixture(),
			contains: []string{
				"# deploy",
				"Branch: `main`",
				"## 1. build.yml",
				"Status: completed (success)",
				"compiling",
				"## 2. deploy.yml",
				"connecting",
			},
		},
		{
			name:    "the failure signature is named once per step",
			runLogs: exportFixture(),
			contains: []string{
				"## Detected issues",
				"**Permission denied** in `deploy.yml`",
			},
			absent: []string{"permission denied again`\n  - `"},
		},
		{
			name:    "an error filter drops the passing step",
			runLogs: exportFixture(),
			config:  errorsOnly,
			contains: []string{
				"## 2. deploy.yml",
				"permission denied to push",
			},
			absent: []string{"## 1. build.yml", "connecting"},
		},
		{
			name:     "a filter matching nothing still says so",
			runLogs:  exportFixture(),
			config:   noMatch,
			contains: []string{"No log lines matched the active filter."},
			absent:   []string{"## 1."},
		},
		{
			name:     "an empty run",
			runLogs:  NewRunLogs("", ""),
			contains: []string{"# Workflow run"},
		},
		{
			name:    "a nil run exports nothing",
			runLogs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ExportAsMarkdown(tt.runLogs, tt.config)
			if err != nil {
				t.Fatalf("ExportAsMarkdown: %v", err)
			}

			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("export is missing %q\n---\n%s", want, got)
				}
			}

			for _, unwanted := range tt.absent {
				if strings.Contains(got, unwanted) {
					t.Errorf("export unexpectedly holds %q\n---\n%s", unwanted, got)
				}
			}
		})
	}
}

func TestExportAsMarkdown_InvalidRegexFilter(t *testing.T) {
	t.Parallel()

	config := NewFilterConfig()
	config.Regex = true
	config.SearchTerm = "("

	if _, err := ExportAsMarkdown(exportFixture(), config); err == nil {
		t.Fatal("expected an error from an uncompilable filter regex")
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		line  string
		label string
	}{
		{"oom", "Process completed with exit code 137", "Out of memory"},
		{"disk", "write /tmp/x: no space left on device", "Out of disk"},
		{"timeout", "The action timed out after 6 hours", "Timeout"},
		{"secret", "Error: missing required secret DEPLOY_KEY", "Missing secret"},
		{"permission", "Resource not accessible by integration", "Permission denied"},
		{"network", "dial tcp 10.0.0.1:443: connection refused", "Network failure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runLogs := NewRunLogs("c", "main")
			runLogs.AddStep(&StepLogs{Workflow: "w.yml", Entries: []LogEntry{{Content: tt.line}}})

			got := Detect(runLogs)
			if len(got) != 1 || got[0].Label != tt.label {
				t.Fatalf("Detect(%q) = %+v, want one %s", tt.line, got, tt.label)
			}
		})
	}
}

func TestDetect_QuietLogsAndNilRun(t *testing.T) {
	t.Parallel()

	runLogs := NewRunLogs("c", "main")
	runLogs.AddStep(&StepLogs{Workflow: "w.yml", Entries: []LogEntry{{Content: "all good"}}})

	if got := Detect(runLogs); len(got) != 0 {
		t.Errorf("Detect on clean logs returned %+v", got)
	}

	if got := Detect(nil); got != nil {
		t.Errorf("Detect(nil) = %+v, want nil", got)
	}
}
