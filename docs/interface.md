# Interface

## Keys

Press `?` for the help modal. It reads the live keymap, so it always matches the binary you are running. Trust it over this page, which lists only enough to start.

| Key                 | Action                                  |
| ------------------- | --------------------------------------- |
| `tab` / `shift+tab` | Move between panes                      |
| `h` / `l`           | Move between the left column and the right panel |
| `[` / `]`           | Move between the right panel's tabs     |
| `j` / `k`           | Move within a pane                      |
| `enter`             | Open the row, or run the highlighted workflow |
| `space`             | Mark the row a verb should act on       |
| `esc`               | Back out one level                      |
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

### Marks

`space` marks the row under the cursor in the workflow list and in the Live tab. A verb then acts on the marked set instead of the selection: `enter` on a marked workflow list confirms every dispatch at once and runs them in order, and `d` on the Live tab stops watching every marked run. With nothing marked each verb acts on the cursor, so marking is an option rather than a step. The action menu names which it is about to do ("run the 3 marked workflows").

Only the selected workflow carries the values in the config pane. The rest of a marked set go out with the defaults they declare, because there is one config pane and a set has many workflows. The confirmation lists every command in full, which is where that shows.

## Panes

The left column stacks what a dispatch is built from, top to bottom: the workflows that declare a `workflow_dispatch` trigger, the chains the repository configures, and the configuration the next run carries. A repository with no chains gets a line naming the feature where the pane would be, and `tab` skips it rather than stopping on an empty one. Opening an input or a history entry in the top-left pane takes the chains pane's rows too, since nothing there acts on a chain.

The right panel runs the full height of the terminal and holds four tabs: Runs, Live, History, and Flaky. `[` and `]` move between them from anywhere, since the right panel is the only tabbed thing on screen. Each tab reports what it holds in the tab bar, so the counts are readable without visiting the tab, and the names abbreviate to their initials on a narrow terminal rather than dropping the counts.

`h` and `l` cross between the columns, and `h` returns to the left pane that had focus rather than to the top of the column.

The config pane takes the height its content needs rather than a fixed share, so a workflow with no inputs leaves the workflow list the rest of the column. Its input table narrows by dropping columns as the column narrows, and the command preview keeps its tail, which is the half carrying the inputs. `c` copies the whole command whatever is on screen.

Selecting a workflow opens its input configuration, built from the input types the workflow declares. Number keys edit an input by position, `r` resets every input to its default, and `c` copies the assembled command. `w` toggles watch mode, which keeps updating the run after dispatch.

The status bar names the global context and nothing a pane already reports: the ref every dispatch targets, and `Chain: name (step/total)` while a chain runs.

After a dispatch the right panel moves to the list that now has something to say: Live when watch is on, History otherwise.

## Runs

The History tab is this checkout's own dispatch history, so in a repository you have never dispatched from it is empty while GitHub holds thousands of runs. The Runs tab answers the other question: what is the current state of each workflow on GitHub.

It loads nothing until opened, and then reads the newest run of each workflow on the branch, keyed on the workflow file *and* its display title, so a workflow that reports a mode in its title (a Pulumi preview against a Pulumi deploy) keeps one current state per mode. Runs older than four hours drop out unless they are still going, falling back to the newest three when nothing at all is that recent, because a repository that dispatches twice a week still has an answer.

`s` cycles the scope between the current branch, your open pull requests, and the pull requests awaiting your review. `R` reloads. `enter` opens the run on a time axis, and the action menu (`a`) adds the run's log and a diagnosis of its failure.

The two pull request scopes list one row per pull request carrying its own check rollup (`2+ 1x`), because that is the exact answer to whether it is green. `enter` on one of those rows expands it into the runs on its head branch, which is where the failing workflow is named. The pane names the ref it drilled into above its rows, and `s` cycles back out.

| Key | Does |
| --- | --- |
| `s` | Next scope: branch, my PRs, awaiting my review |
| `R` | Reload the current scope |
| `enter` | Open the selected run on a time axis, or expand a pull request into its branch's runs |
| `a` | Actions for the selected run, including its log and a diagnosis of its failure |

`:runs [branch\|mine\|reviewing]` opens a scope by name.

## Timeline

Opening a run replaces the list with that run's jobs drawn as bars on one shared axis, which is what a list of statuses cannot show: what ran at the same time, and where the wall clock went. A run whose slowest job succeeded while a fast one failed reads identically in a status list and obviously here.

```
Runs › ci  [esc] back
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

It is a drill-down of the row that names the run rather than a tab beside it, so the breadcrumb says which list it came from. `enter` drills further into the selected job's steps, rescaling the axis to that job's own window. `esc` peels one layer per press: a job's steps, then the run, then whatever view you were in.

The action menu (`a`) reaches the run's log and a diagnosis from here, so the timeline names the failing job and the log says why.

Open one with `enter` on a Runs, Live, or Flaky row, or with `:timeline`, which draws whatever is selected and takes a run id to draw one by number. A bar with no end yet is drawn open (`▓`) against a clock that runs to now, and redraws once a second while the run is going.

## Flaky

Per-branch, one run per workflow is the whole answer. Whether a workflow is *reliable* is the opposite query, so it gets its own tab: many runs of one workflow rather than one run of each.

It reads one page of the repository's recent runs and derives both of its views from that single listing. With `all workflows` selected on the left it groups them by workflow, flakiest first:

```
every workflow · flakiest first
    Workflow                Runs  Pass  Last
> x Configured Graph Update   30   63%  49m ago
  + CI                        42   88%  8m ago
  + Bump Version              12  100%  8m ago
```

A workflow whose runs have all yet to finish has no rate rather than a zero one, and sorts after every measured one: unknown is not the same answer as failing.

Selecting a workflow on the left narrows the tab to that workflow's own runs, naming the branch and event behind each, which costs no second API call. `enter` there opens the run on a time axis. `R` re-reads the page.

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
