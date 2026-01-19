package app

import (
	"testing"
)

func TestPadRight(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		length int
		want   string
	}{
		{"shorter than length", "hello", 10, "hello     "},
		{"equal to length", "hello", 5, "hello"},
		{"longer than length", "hello world", 5, "hello world"},
		{"empty string", "", 5, "     "},
		{"zero length", "hello", 0, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := _padRight(tt.input, tt.length)
			if got != tt.want {
				t.Errorf("_padRight(%q, %d) = %q, want %q", tt.input, tt.length, got, tt.want)
			}
		})
	}
}

func TestFormatRowNumber(t *testing.T) {
	tests := []struct {
		index int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{8, "8"},
		{9, "9"},
		{10, " "},
		{100, " "},
	}

	for _, tt := range tests {
		got := _formatRowNumber(tt.index)
		if got != tt.want {
			t.Errorf("_formatRowNumber(%d) = %q, want %q", tt.index, got, tt.want)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		slice  []string
		item   string
		want   bool
	}{
		{"item exists", []string{"a", "b", "c"}, "b", true},
		{"item not exists", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"empty item in slice", []string{"", "b"}, "", true},
		{"case sensitive", []string{"Hello"}, "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := _contains(tt.slice, tt.item)
			if got != tt.want {
				t.Errorf("_contains(%v, %q) = %v, want %v", tt.slice, tt.item, got, tt.want)
			}
		})
	}
}

func TestWordWrap(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  string
	}{
		{
			"basic wrap",
			"hello world foo bar",
			10,
			"hello\nworld foo\nbar",
		},
		{
			"no wrap needed",
			"hello",
			10,
			"hello",
		},
		{
			"exact width",
			"hello world",
			11,
			"hello world",
		},
		{
			"single word longer than width",
			"supercalifragilisticexpialidocious",
			10,
			"supercalifragilisticexpialidocious",
		},
		{
			"zero width",
			"hello world",
			0,
			"hello world",
		},
		{
			"negative width",
			"hello world",
			-1,
			"hello world",
		},
		{
			"multiple spaces",
			"hello   world",
			10,
			"hello\nworld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := _wordWrap(tt.text, tt.width)
			if got != tt.want {
				t.Errorf("_wordWrap(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
			}
		})
	}
}
