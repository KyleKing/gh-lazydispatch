package browser

import (
	"runtime"
	"testing"
)

func TestOpen(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	// Test with a valid URL
	// Note: This won't actually open a browser in CI, but will verify the command
	// can be constructed and started without error
	url := "https://example.com"

	err := Open(url)
	if err != nil {
		t.Errorf("Open failed: %v", err)
	}
}

func TestOpen_InvalidURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	// Test with an invalid URL format
	// The browser command may still succeed since it's just passed to the shell
	url := "not a valid url"

	err := Open(url)
	// We don't expect an error here because the command starts successfully
	// even with an invalid URL - the browser will handle the invalid URL
	if err != nil {
		t.Logf("Open with invalid URL returned error (expected in some cases): %v", err)
	}
}

func TestOpen_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("skipping macOS-specific test")
	}

	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	url := "https://example.com"
	err := Open(url)

	if err != nil {
		t.Errorf("Open on macOS failed: %v", err)
	}
}

func TestOpen_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping Linux-specific test")
	}

	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	url := "https://example.com"
	err := Open(url)

	if err != nil {
		t.Errorf("Open on Linux failed: %v", err)
	}
}

func TestOpen_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows-specific test")
	}

	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	url := "https://example.com"
	err := Open(url)

	if err != nil {
		t.Errorf("Open on Windows failed: %v", err)
	}
}

func TestOpen_EmptyURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	err := Open("")

	// Empty URL should still work - command will start
	if err != nil {
		t.Logf("Open with empty URL returned error: %v", err)
	}
}
