package app

import (
	"testing"
)

func TestFormatRowNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		index int
		want  string
	}{
		{0, "1"},
		{1, "2"},
		{8, "9"},
		{9, "0"},
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
	t.Parallel()

	tests := []struct {
		name  string
		slice []string
		item  string
		want  bool
	}{
		{"item exists", []string{"a", "b", "c"}, "b", true},
		{"item not exists", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"empty item in slice", []string{"", "b"}, "", true},
		{"case sensitive", []string{"Hello"}, "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := _contains(tt.slice, tt.item)
			if got != tt.want {
				t.Errorf("_contains(%v, %q) = %v, want %v", tt.slice, tt.item, got, tt.want)
			}
		})
	}
}

func TestWordWrap(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			got := _wordWrap(tt.text, tt.width)
			if got != tt.want {
				t.Errorf("_wordWrap(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
			}
		})
	}
}
