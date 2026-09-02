package github_test

import (
	"errors"
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/testutil"
)

// errRefused stands in for a token that cannot read the environments.
var errRefused = errors.New("HTTP 403")

// An environment input's values are the repository's environments, so the
// listing has to survive the shapes gh actually answers with: a name per line,
// nothing at all, and a token that cannot read them.
func TestListEnvironments_ReadsTheNamesAndSurvivesAnEmptyOrRefusedAnswer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stdout string
		err    error
		want   []string
		wantOK bool
	}{
		{name: "one per line", stdout: "production\nstaging\n", want: []string{"production", "staging"}, wantOK: true},
		{name: "blank lines between pages", stdout: "\nproduction\n\n", want: []string{"production"}, wantOK: true},
		{name: "a repository with none", stdout: "", wantOK: true},
		{name: "a token that cannot read them", err: errRefused},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := testutil.NewMockExecutor()
			mock.AddCommand("gh", []string{
				"api", "--paginate",
				"repos/owner/repo/environments", "--jq", ".environments[].name",
			}, tt.stdout, "", tt.err)

			client, err := github.NewClientWithExecutor("owner/repo", mock)
			if err != nil {
				t.Fatal(err)
			}

			names, err := client.ListEnvironments()
			if (err == nil) != tt.wantOK {
				t.Fatalf("error is %v, want ok=%v", err, tt.wantOK)
			}

			if len(names) != len(tt.want) {
				t.Fatalf("read %v, want %v", names, tt.want)
			}

			for i, want := range tt.want {
				if names[i] != want {
					t.Errorf("name %d is %q, want %q", i, names[i], want)
				}
			}
		})
	}
}
