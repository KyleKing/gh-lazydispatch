package git

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	historyTimeout   = 10 * time.Second
	historyFixedArgs = 6
)

// Change is one commit that touched a path.
type Change struct {
	At      time.Time
	Subject string
}

// PathChanges lists the commits on ref since the given time that touched any of
// paths, oldest first. It shells out to git directly because the vcs layer has
// no path-scoped log.
func PathChanges(ctx context.Context, ref string, since time.Time, paths ...string) ([]Change, error) {
	ctx, cancel := context.WithTimeout(ctx, historyTimeout)
	defer cancel()

	args := make([]string, 0, historyFixedArgs+len(paths))
	args = append(args, "log", "--reverse", "--since="+since.Format(time.RFC3339), "--format=%cI\t%s", ref, "--")
	args = append(args, paths...)

	out, err := exec.CommandContext(ctx, "git", args...).Output() //nolint:gosec // fixed subcommand
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", ref, err)
	}

	var changes []Change

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		stamp, subject, _ := strings.Cut(scanner.Text(), "\t")

		at, err := time.Parse(time.RFC3339, stamp)
		if err != nil {
			return nil, fmt.Errorf("parsing commit time %q: %w", stamp, err)
		}

		changes = append(changes, Change{At: at.UTC(), Subject: subject})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading git log: %w", err)
	}

	return changes, nil
}
