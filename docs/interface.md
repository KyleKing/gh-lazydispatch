# Interface

## Keys

Press `?` for the help modal. It reads the live keymap, so it always matches the binary you are running. Trust it over this page, which lists only enough to start.

| Key                 | Action                                  |
| ------------------- | --------------------------------------- |
| `tab` / `shift+tab` | Move between panes                      |
| `j` / `k`           | Move within a pane                      |
| `enter`             | Select, or run the highlighted workflow |
| `a`                 | Actions for whatever has focus          |
| `:`                 | Command bar                             |
| `/`                 | Filter                                  |
| `?`                 | Help                                    |
| `q` or `ctrl+c`     | Quit                                    |

`gh-lazydispatch --help` also prints a shortcut summary without starting the TUI.

### Finding a verb you do not know the key for

Two ways in, which are two different grammars.

`a` opens the verbs that apply to whatever has focus, and only those: remapping a history entry's stale inputs is offered while you are previewing one and absent otherwise, rather than bound to a key that silently does nothing. Each verb keeps its own key, so the menu teaches the keys instead of replacing them.

`:` opens a command bar. Commands have names rather than letters, `tab` completes as far as the candidates agree, and an ambiguous prefix lists what it could not choose between. `:branch`, `:chain`, and `:workflow` complete over the branches, chains, and workflows this repository actually has.

The frequent keys stay direct: `b` for branch, `w` for watch, `r` for reset, `c` to copy the command, digits to jump.

## Panes

The left pane lists the workflows that declare a `workflow_dispatch` trigger. The right pane is tabbed, holding History, Chains, and Live runs. `h` and `l` move between those tabs once the right pane has focus.

Selecting a workflow opens its input configuration, built from the input types the workflow declares. Number keys edit an input by position, `r` resets every input to its default, and `c` copies the assembled command to the clipboard. `w` toggles watch mode, which keeps updating the run after dispatch.

The status bar shows `Chains(N)` when the repository has chains configured, and `Chain: name (step/total)` while one runs.

## Log viewer

`v` opens logs from a chain status screen or from a history entry. Logs arrive organized by workflow step, one tab per step.

Inside the viewer:

| Key                 | Action                                             |
| ------------------- | -------------------------------------------------- |
| `tab` / `shift+tab` | Move between step tabs                             |
| `f`                 | Cycle the filter through all, errors, and warnings |
| `/`                 | Search                                             |
| `n` / `N`           | Next or previous match                             |
| `i`                 | Toggle case sensitivity                            |
| `a` / `w` / `e`     | Filter to all, warnings, or errors                 |
| `enter` / `space`   | Fold or unfold the step under the cursor           |
| `E` / `C`           | Unfold or fold every step                          |
| `s`                 | Toggle auto-scroll while streaming                 |
| `x`                 | Export what the filter kept to a markdown file     |
| `o`                 | Open the run in a browser                          |
| `q` or `esc`        | Close the viewer                                   |

Logs keep streaming while a run is active, and opening the viewer from a failed chain starts it filtered to errors.

`x` writes `lazydispatch-<name>-<timestamp>.md` into the working directory: one
section per step with its status, the log lines the active filter kept, and a
"Detected issues" list naming the failure signatures found (out of memory, out
of disk, timeout, missing secret, permission denied, network failure) with what
to try about each. The footer reports the path it wrote.

Log viewing needs the `gh` CLI installed and logged in.
