# Interface

## Keys

Press `?` for the help modal. It reads the live keymap, so it always matches the binary you are running. Trust it over this page, which lists only enough to start.

| Key                 | Action                                  |
| ------------------- | --------------------------------------- |
| `tab` / `shift+tab` | Move between panes                      |
| `j` / `k`           | Move within a pane                      |
| `enter`             | Select, or run the highlighted workflow |
| `b`                 | Choose a branch                         |
| `/`                 | Filter                                  |
| `?`                 | Help                                    |
| `q` or `ctrl+c`     | Quit                                    |

`gh-lazydispatch --help` also prints a shortcut summary without starting the TUI.

## Panes

The left pane lists the workflows that declare a `workflow_dispatch` trigger. The right pane is tabbed, holding History, Chains, and Live runs. `h` and `l` move between those tabs once the right pane has focus.

Selecting a workflow opens its input configuration, built from the input types the workflow declares. Number keys edit an input by position, `r` resets every input to its default, and `c` copies the assembled command to the clipboard. `w` toggles watch mode, which keeps updating the run after dispatch.

The status bar shows `Chains(N)` when the repository has chains configured, and `Chain: name (step/total)` while one runs.

## Log viewer

`l` opens logs from a chain status screen or from a history entry. Logs arrive organized by workflow step, one tab per step.

Inside the viewer:

| Key                 | Action                                             |
| ------------------- | -------------------------------------------------- |
| `tab` / `shift+tab` | Move between step tabs                             |
| `f`                 | Cycle the filter through all, errors, and warnings |
| `/`                 | Search                                             |
| `n` / `N`           | Next or previous match                             |
| `i`                 | Toggle case sensitivity                            |
| `o`                 | Open the run in a browser                          |
| `q` or `esc`        | Close the viewer                                   |

Logs keep streaming while a run is active, and opening the viewer from a failed chain starts it filtered to errors.

Log viewing needs the `gh` CLI installed and logged in.
