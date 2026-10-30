package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/telemetry"
)

const hoursPerDay = 24

type telemetryRow struct {
	LastSeen  string `json:"last_seen"`
	FirstSeen string `json:"first_seen"`
	Name      string `json:"name"`
	Count     int    `json:"count"`
	IdleDays  int    `json:"idle_days"`
}

// exportTelemetry reads back the local usage counts: which commands, verbs, and
// dispatches get used, and how long each has sat idle.
func exportTelemetry(args []string, stdout, _ io.Writer) error {
	fs := newFlagSet("telemetry")

	var format string

	fs.StringVar(&format, "format", formatJSONName, "json or md")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	store, err := telemetry.Load(telemetry.CachePath())
	if err != nil {
		return fmt.Errorf("reading usage counts: %w", err)
	}

	rows := telemetryRows(store, time.Now().UTC())

	if format == "md" {
		_, err := io.WriteString(stdout, telemetryMarkdown(rows))
		if err != nil {
			return fmt.Errorf("writing markdown: %w", err)
		}

		return nil
	}

	return writeJSON(stdout, rows)
}

const formatJSONName = "json"

func telemetryRows(store *telemetry.Store, now time.Time) []telemetryRow {
	rows := make([]telemetryRow, 0, len(store.Events))

	for name, e := range store.Events {
		rows = append(rows, telemetryRow{
			Name: name, Count: e.Count, FirstSeen: e.FirstSeen.Format(time.DateOnly),
			LastSeen: e.LastSeen.Format(time.DateOnly), IdleDays: int(now.Sub(e.LastSeen).Hours() / hoursPerDay),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}

		return rows[i].Name < rows[j].Name
	})

	return rows
}

func telemetryMarkdown(rows []telemetryRow) string {
	var b strings.Builder

	b.WriteString("| Feature | Uses | First used | Last used | Idle days |\n| --- | ---: | --- | --- | ---: |\n")

	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %d | %s | %s | %d |\n", r.Name, r.Count, r.FirstSeen, r.LastSeen, r.IdleDays)
	}

	return b.String()
}
