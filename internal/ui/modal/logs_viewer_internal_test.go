package modal

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

func createTestRunLogs() *logs.RunLogs {
	return &logs.RunLogs{
		ChainName: "test-chain",
		Branch:    "main",
		Steps: []*logs.StepLogs{
			{
				StepIndex: 0,
				StepName:  "Setup",
				Workflow:  "test.yml",
				RunID:     12345,
				JobName:   "test-job",
				Status:    "completed",
				Entries: []logs.LogEntry{
					{Timestamp: time.Now(), Content: "Starting setup", Level: logs.LogLevelInfo},
					{Timestamp: time.Now(), Content: "Setup complete", Level: logs.LogLevelInfo},
				},
			},
			{
				StepIndex: 1,
				StepName:  "Build",
				Workflow:  "test.yml",
				RunID:     12345,
				JobName:   "test-job",
				Status:    "completed",
				Entries: []logs.LogEntry{
					{Timestamp: time.Now(), Content: "Building project", Level: logs.LogLevelInfo},
					{Timestamp: time.Now(), Content: "Warning: deprecated API", Level: logs.LogLevelWarning},
					{Timestamp: time.Now(), Content: "Build failed: syntax error", Level: logs.LogLevelError},
				},
			},
		},
	}
}

func TestLogsViewerModal_Creation(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	if modal == nil {
		t.Fatal("expected non-nil modal")
	}

	if modal.IsDone() {
		t.Error("modal should not be done initially")
	}

	if modal.IsStreaming() {
		t.Error("modal should not be streaming initially")
	}
}

func TestLogsViewerModal_CreationWithError(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModalWithError(runLogs, 80, 24)

	if modal == nil {
		t.Fatal("expected non-nil modal")
	}

	// Error mode should filter to errors only
	if modal.filterCfg.Level != logs.FilterErrors {
		t.Errorf("expected error filter, got %v", modal.filterCfg.Level)
	}
}

func TestLogsViewerModal_View(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	view := modal.View()

	if view == "" {
		t.Error("expected non-empty view")
	}

	// View should contain step names
	if !strings.Contains(view, "Setup") && !strings.Contains(view, "Build") {
		t.Error("view should contain step names")
	}
}

func TestLogsViewerModal_Close(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	// Press q to close
	_, _ = modal.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})

	if !modal.IsDone() {
		t.Error("modal should be done after q key")
	}
}

func TestLogsViewerModal_CloseEscape(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	// Press esc to close
	_, _ = modal.Update(tea.KeyPressMsg{Code: tea.KeyEsc})

	if !modal.IsDone() {
		t.Error("modal should be done after esc key")
	}
}

func TestLogsViewerModal_ToggleFilter(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	initialLevel := modal.filterCfg.Level

	// Press f to toggle filter
	_, _ = modal.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})

	if modal.filterCfg.Level == initialLevel {
		t.Error("filter level should have changed after f key")
	}
}

func TestLogsViewerModal_QuickFilters(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	// Press e for errors only
	_, _ = modal.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})

	if modal.filterCfg.Level != logs.FilterErrors {
		t.Error("expected error filter after e key")
	}

	// Press w for warnings
	_, _ = modal.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})

	if modal.filterCfg.Level != logs.FilterWarnings {
		t.Error("expected warning filter after w key")
	}

	// Press a for all
	_, _ = modal.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})

	if modal.filterCfg.Level != logs.FilterAll {
		t.Error("expected all filter after a key")
	}
}

func TestLogsViewerModal_SearchMode(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	// Press / to enter search mode
	_, _ = modal.Update(tea.KeyPressMsg{Code: '/', Text: "/"})

	if !modal.searchMode {
		t.Error("expected search mode after / key")
	}

	// Exit search mode with esc
	_, _ = modal.Update(tea.KeyPressMsg{Code: tea.KeyEsc})

	// Search mode should be off but modal should not be done
	if modal.searchMode {
		t.Error("search mode should be off after esc")
	}
}

func TestLogsViewerModal_ToggleCaseSensitivity(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	initialCaseSensitive := modal.filterCfg.CaseSensitive

	// Press i to toggle case sensitivity
	_, _ = modal.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})

	if modal.filterCfg.CaseSensitive == initialCaseSensitive {
		t.Error("case sensitivity should have toggled after i key")
	}
}

func TestLogsViewerModal_Resize(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	// Resize the modal
	_, _ = modal.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if modal.width != 120 {
		t.Errorf("expected width 120, got %d", modal.width)
	}

	if modal.height != 40 {
		t.Errorf("expected height 40, got %d", modal.height)
	}
}

func TestLogsViewerModal_Result(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	result := modal.Result()

	if result != nil {
		t.Error("expected nil result for logs viewer modal")
	}
}

func TestLogsViewerModal_CollapseExpand(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	// Press C to collapse all
	_, _ = modal.Update(tea.KeyPressMsg{Code: 'C', Text: "C"})

	for stepIdx := range runLogs.Steps {
		if !modal.collapsedSteps[stepIdx] {
			t.Errorf("step %d should be collapsed after C key", stepIdx)
		}
	}

	// Press E to expand all
	_, _ = modal.Update(tea.KeyPressMsg{Code: 'E', Text: "E"})

	for stepIdx := range runLogs.Steps {
		if modal.collapsedSteps[stepIdx] {
			t.Errorf("step %d should be expanded after E key", stepIdx)
		}
	}
}

func TestLogsViewerModal_Streaming(t *testing.T) {
	t.Parallel()

	runLogs := createTestRunLogs()
	modal := NewLogsViewerModal(runLogs, 80, 24)

	modal.EnableStreaming(12345, true)

	if !modal.IsStreaming() {
		t.Error("expected streaming to be enabled")
	}

	if modal.streamRunID != 12345 {
		t.Errorf("expected stream run ID 12345, got %d", modal.streamRunID)
	}

	if !modal.autoScroll {
		t.Error("expected autoScroll to be enabled")
	}
}

//nolint:paralleltest // t.Chdir rules out t.Parallel
func TestLogsViewerModal_ExportWritesMarkdownToTheWorkingDirectory(t *testing.T) {
	t.Chdir(t.TempDir())

	modal := NewLogsViewerModal(createTestRunLogs(), 80, 24)

	_, _ = modal.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})

	if !strings.HasPrefix(modal.exportStatus, "exported to ") {
		t.Fatalf("export status = %q, want an exported-to path", modal.exportStatus)
	}

	path := strings.TrimPrefix(modal.exportStatus, "exported to ")

	written, err := os.ReadFile(path) //nolint:gosec // the path is the one the modal just reported writing
	if err != nil {
		t.Fatalf("reading the export: %v", err)
	}

	if !strings.Contains(string(written), "```") {
		t.Errorf("export carries no fenced log body:\n%s", written)
	}

	if !strings.Contains(modal.View(), "exported to ") {
		t.Error("the footer does not report the export")
	}
}
