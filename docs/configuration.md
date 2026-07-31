# Configuration

## Config file

`.github/lazydispatch.yml` in the repository defines workflow chains. Nothing else is configurable through a file, and a repository without one simply shows no Chains tab entries. See [chains](./chains.md) for the schema.

## Environment variables

| Variable           | Effect                                    |
| ------------------ | ----------------------------------------- |
| `CATPPUCCIN_THEME` | Force the theme to `latte` or `macchiato` |

Without the override, the theme follows the terminal background.

## Flags

`-h` or `--help` prints usage, the shortcut summary, and the environment variables. `-v` or `--version` prints the version with its commit and build date. There are no others, because every choice happens inside the TUI.
