package testrepo

import (
	"errors"
	"strings"
	"testing"
)

var errUnreadable = errors.New("HTTP 404")

// recorder captures what the guard refused. A guard that fails open is worse
// than no guard, since it reads as protection while publishing.
type recorder struct {
	message string
	failed  bool
}

func (*recorder) Helper() {}

func (r *recorder) Fatalf(format string, _ ...any) {
	r.failed = true
	r.message = format
}

// Only PUBLIC may be recorded. Everything else, including a repository gh
// cannot read at all, has to stop the recording rather than fall through it.
func TestRequirePublic_RecordsNothingItCannotPublish(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err        error
		name       string
		visibility string
		allowed    bool
	}{
		{name: "public", visibility: "PUBLIC", allowed: true},
		{name: "private", visibility: "PRIVATE"},
		{name: "internal", visibility: "INTERNAL"},
		{name: "unreadable", err: errUnreadable},
		{name: "an empty answer", visibility: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recorder{}
			refuseUnlessPublic(r, "owner/repo", func(string) (string, error) {
				return tt.visibility, tt.err
			})

			if r.failed == tt.allowed {
				t.Errorf("visibility %q err %v: refused=%v, want refused=%v: %s",
					tt.visibility, tt.err, r.failed, !tt.allowed, r.message)
			}
		})
	}
}

// The refusal has to name the repository and why, since it fires during a
// manual re-record where the reader has to decide what to do next.
func TestRequirePublic_SaysWhatItRefusedAndWhy(t *testing.T) {
	t.Parallel()

	r := &recorder{}
	refuseUnlessPublic(r, "owner/secret", func(string) (string, error) { return "PRIVATE", nil })

	for _, want := range []string{"refusing to record", "committed verbatim"} {
		if !strings.Contains(r.message, want) {
			t.Errorf("the refusal reads %q, missing %q", r.message, want)
		}
	}
}
