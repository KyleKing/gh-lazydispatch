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

The left pane lists the workflows that declare a `workflow_dispatch` trigger. The right pane is tabbed, holding History, Chains, Live, Timeline, and Runs. `h` and `l` move between those tabs once the right pane has focus. Each tab reports what it holds in the tab bar, so the counts are readable without visiting the tab, and the names abbreviate to their initials on a narrow terminal rather than dropping the counts.

The config pane takes the height its content needs rather than half the screen, so a workflow with no inputs leaves the lists above it the rest of the terminal.

Selecting a workflow opens its input configuration, built from the input types the workflow declares. Number keys edit an input by position, `r` resets every input to its default, and `c` copies the assembled command to the clipboard. `w` toggles watch mode, which keeps updating the run after dispatch.

The status bar names the branch and, once the Runs tab has loaded, its verdict: `main 12+ 1x` is twelve passing workflows and one failing. It also shows `Chains(N)` when the repository has chains configured, and `Chain: name (step/total)` while one runs.

## Runs

The History tab is this checkout's own dispatch history, so in a repository you have never dispatched from it is empty while GitHub holds thousands of runs. The Runs tab answers the other question: what is the current state of each workflow on GitHub.

It loads nothing until opened, and then reads the newest run of each workflow on the branch, keyed on the workflow file *and* its display title, so a workflow that reports a mode in its title (a Pulumi preview against a Pulumi deploy) keeps one current state per mode. Runs older than four hours drop out unless they are still going, falling back to the newest three when nothing at all is that recent, because a repository that dispatches twice a week still has an answer.

`s` cycles the scope between the current branch, your open pull requests, and the pull requests awaiting your review. `R` reloads. `enter` opens the run's log, and the action menu (`a`) adds diagnose and timeline.

| Key | Does |
| --- | --- |
| `s` | Next scope: branch, my PRs, awaiting my review |
| `R` | Reload the current scope |
| `enter` | Open the selected run's log |
| `a` then `d` | Diagnose the selected run's failure |
| `a` then `t` | Draw the selected run on the timeline |

`:runs [branch\|mine\|reviewing]` opens a scope by name.

The two pull request scopes read one page of the repository's recent runs rather than a page per branch, so a pull request whose last run has aged off that page reports nothing rather than costing another round trip.

## Timeline

The fourth right-panel tab draws a run's jobs as bars on one shared axis, which is what a list of statuses cannot show: what ran at the same time, and where the wall clock went. A run whose slowest job succeeded while a fast one failed reads identically in a status list and obviously here.

```
run 33423560774
> ✓ actionlint           ███                                           10s
  ✓ project            █████                                           15s
  ✓ benchmark                ██████████████████████████████████████  1m49s
  ✗ ci                  ██████████████████████████████              1m26s
  ✓ lint                     █████████████████████████████████       1m35s
  ✓ hooks                    ██████████                                29s
                       └──────────────────────────────────────────┘
                       0                                      2m07s
```

`ci` failed at 1m26s, and `benchmark` set the run's 2m07s. Every bar is measured against the same window, so two bars of the same length took the same time.

`enter` drills into the selected job's steps, rescaling the axis to that job's own window; `esc` backs out to the jobs. Escape peels one layer at a time, so backing out of a job does not also leave whatever view you were in.

Fill it with `a` then `t` on a History or Live row, or with `:timeline <run-id>`. A bar with no end yet is drawn open (`▓`) against a clock that runs to now.

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
