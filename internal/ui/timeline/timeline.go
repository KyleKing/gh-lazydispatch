// Package timeline lays out a workflow run as bars on a shared time axis. It
// answers the two questions a list of statuses cannot: what ran at the same
// time, and where the wall clock went. A run whose slowest job succeeded while
// a fast one failed looks the same in a status list and obvious here.
//
// The package holds no colors and no terminal vocabulary. It measures spans
// and returns rows of cells, so the caller keeps naming its own styles.
package timeline

import (
	"time"
)

// Span is one bar: something that started, possibly finished, and has an
// outcome worth marking.
type Span struct {
	Start time.Time
	// End is the zero time for something still running, which is drawn to the
	// right edge of whatever window the rest of the run defines.
	End        time.Time
	Label      string
	Conclusion string
	// Children are the spans one level down, which the caller drills into.
	Children []Span
}

// Row is one laid-out bar: where it starts, how wide it is, and how long it
// took. Offset and Width are in terminal cells within the track.
type Row struct {
	Label      string
	Conclusion string
	Duration   time.Duration
	Offset     int
	Width      int
	// Running marks a span with no end, drawn open rather than closed.
	Running bool
}

// Layout is a set of rows sharing one axis.
type Layout struct {
	Rows []Row
	// Span is the wall clock the axis covers, which is the whole run rather
	// than the sum of its parts.
	Span time.Duration
	// Track is the width in cells the rows were laid out against.
	Track int
}

// minTrack is the narrowest axis worth drawing. Below it every bar rounds to
// the same cell and the picture says nothing.
const minTrack = 8

// Lay places spans on a shared axis track cells wide. The window runs from the
// earliest start to the latest end, so offsets are comparable across rows;
// that shared window is the whole point, and laying each row out against its
// own duration would make a 10-second job look like a 2-minute one.
//
// A span still running is closed at now; a zero now closes it at the latest
// end instead, which is what a completed run wants.
func Lay(spans []Span, track int, now time.Time) Layout {
	if len(spans) == 0 || track < minTrack {
		return Layout{Track: track}
	}

	start, end := window(spans, now)

	span := end.Sub(start)
	if span <= 0 {
		span = time.Second
	}

	rows := make([]Row, 0, len(spans))

	for _, s := range spans {
		rows = append(rows, layRow(s, start, span, track, end))
	}

	return Layout{Rows: rows, Span: span, Track: track}
}

func layRow(s Span, start time.Time, span time.Duration, track int, end time.Time) Row {
	finish := s.End
	running := s.End.IsZero()

	if running {
		finish = end
	}

	offset := cell(s.Start.Sub(start), span, track)
	width := cell(finish.Sub(start), span, track) - offset

	// Every span that happened occupies at least one cell, so a step too short
	// to measure still shows that it ran.
	if width < 1 {
		width = 1
	}

	if offset+width > track {
		offset = track - width
	}

	if offset < 0 {
		offset = 0
	}

	return Row{
		Label:      s.Label,
		Conclusion: s.Conclusion,
		Duration:   finish.Sub(s.Start),
		Offset:     offset,
		Width:      width,
		Running:    running,
	}
}

// window is the earliest start and latest end across spans, with anything
// still running closed at now.
func window(spans []Span, now time.Time) (time.Time, time.Time) {
	var start, end time.Time

	for _, s := range spans {
		if start.IsZero() || s.Start.Before(start) {
			start = s.Start
		}

		finish := s.End
		if finish.IsZero() {
			finish = now
		}

		if finish.After(end) {
			end = finish
		}
	}

	if end.Before(start) {
		end = start
	}

	return start, end
}

// cell converts an offset into the window to a column in the track.
func cell(d, span time.Duration, track int) int {
	if d <= 0 {
		return 0
	}

	c := int(float64(d) / float64(span) * float64(track))
	if c > track {
		return track
	}

	return c
}
