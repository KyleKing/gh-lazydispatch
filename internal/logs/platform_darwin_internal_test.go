//go:build darwin

package logs

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDarwin_CachePath(t *testing.T) {
	// Test macOS-specific cache path handling
	cache := NewCache(t.TempDir())

	if cache.cacheDir == "" {
		t.Error("expected non-empty cache dir")
	}

	// Verify path separators are correct for macOS
	if filepath.Separator != '/' {
		t.Error("expected forward slash separator on macOS")
	}
}

func TestDarwin_LogParsing(t *testing.T) {
	// Test log parsing on macOS
	rawLogs := "##[group]Test\nINFO: macOS test\n##[endgroup]"
	entries := ParseLogOutput(rawLogs, "test")

	if len(entries) == 0 {
		t.Error("expected parsed entries")
	}
}

func TestDarwin_FileOperations(t *testing.T) {
	// Test file operations work correctly on macOS
	cacheDir := t.TempDir()
	cache := NewCache(cacheDir)

	runLogs := NewRunLogs("test", "main")
	runLogs.AddStep(&StepLogs{StepName: "build"})

	err := cache.Put("test", 123, runLogs, 1*time.Hour)
	if err != nil {
		t.Errorf("Put failed on macOS: %v", err)
	}

	_, found := cache.Get("test", 123)
	if !found {
		t.Error("expected to find cached entry on macOS")
	}
}
