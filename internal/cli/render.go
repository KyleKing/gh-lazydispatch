package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

// timeFormat is RFC 3339 without sub-second precision, which is all the
// Actions API carries and all a caller sorts on.
const timeFormat = "2006-01-02T15:04:05Z07:00"

// writeJSON writes v as indented JSON with a trailing newline, so output is
// readable in a terminal and still valid input to jq.
func writeJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(v); err != nil {
		return fmt.Errorf("writing JSON: %w", err)
	}

	return nil
}
