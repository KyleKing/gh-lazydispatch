package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/kyleking/aragonite/keyusage"

	"github.com/kyleking/gh-lazydispatch/internal/app"
)

// exportKeys reads the key usage log back against the keymap.
func exportKeys(args []string, stdout, _ io.Writer) error {
	fs := newFlagSet("keys")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	dir := keyusage.Dir(app.UsageApp)
	if dir == "" {
		_, err := fmt.Fprintf(stdout, "no usage log: set %s or create ~/Sync\n", keyusage.EnvVar(app.UsageApp))

		return err //nolint:wrapcheck // a stdout write failure needs no context
	}

	km := app.Keymap()
	km[keyusage.CLI] = keyusage.Commands(Root())

	u, err := keyusage.Load(dir, km)
	if err != nil {
		return fmt.Errorf("reading the usage log: %w", err)
	}

	if err := keyusage.Write(stdout, u, km, time.Now().Add(-app.StaleAfter)); err != nil {
		return fmt.Errorf("writing the key report: %w", err)
	}

	return nil
}
