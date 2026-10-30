package listener

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// bell is the terminal bell, which most terminals turn into a dock bounce or a
// tab badge, so a watcher you are not looking at still gets your attention.
const bell = "\a"

// Notify tells the person the outcome: a bell on w and, when cmd is set, cmd run
// with the summary as its one argument. A failing command is returned, because a
// notification that silently did not arrive is the failure a watcher exists to
// prevent.
func Notify(ctx context.Context, w io.Writer, cmd string, outcome Outcome) error {
	if _, err := io.WriteString(w, bell); err != nil {
		return fmt.Errorf("ringing the terminal bell: %w", err)
	}

	if cmd == "" {
		return nil
	}

	//nolint:gosec // cmd is an operator-supplied flag, same trust boundary as $EDITOR
	if err := exec.CommandContext(ctx, cmd, outcome.Summary).Run(); err != nil {
		return fmt.Errorf("running %s: %w", cmd, err)
	}

	return nil
}
