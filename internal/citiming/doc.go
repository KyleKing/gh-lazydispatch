// Package citiming measures how long CI takes and where the time goes: per-push
// wall clock, epochs cut at pipeline changes, job-level paths, runner queues,
// spend, flakiness, and merge queue behavior. It reads GitHub Actions through
// internal/github and holds no terminal or flag vocabulary.
package citiming

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// timeFormat is how timestamps are written into the summaries.
const timeFormat = time.RFC3339

// notef writes progress to stderr. A caller redirecting stdout to a file has
// nowhere useful to report a broken stderr, so the write is deliberately unchecked.
func notef(stderr io.Writer, format string, args ...any) {
	//nolint:errcheck // diagnostics on stderr; nothing to do if they fail
	fmt.Fprintf(stderr, format, args...)
}

func writeJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(v); err != nil {
		return fmt.Errorf("writing JSON: %w", err)
	}

	return nil
}
