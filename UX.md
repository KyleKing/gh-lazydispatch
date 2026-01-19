# Configuration Panel UX

## Layout

Four-region layout with status bar:
- **Status bar** (top): Shows chains count, live runs, chain execution status
- **Top-left**: Workflow list (or Input Details when an input is selected)
- **Top-right**: Tabbed panel (History, Chains, Live runs)
- **Bottom**: Configuration panel with table-based input display

```
+-------------------------------------------------------------------------+
|  Chains(2)  Live(3*)  Chain: deploy (2/3)                  lazydispatch |
+- Workflows ---------------+- [History] Chains  Live ---------------------+
| > deploy.yml              | > main                          2m ago       |
|   test.yml                |   feature/auth                  1h ago       |
|   build.yml               |   develop                       3h ago       |
|                           |                                              |
+---------------------------+----------------------------------------------+
| Configuration: deploy.yml                                                |
| Branch: [b] main    Watch: [w] on    [r] reset all                       |
| ...inputs table...                                                       |
| [Tab] pane  [Enter] run  [h/l] tab  [?] help  [q] quit                   |
+--------------------------------------------------------------------------+
```

## Status Bar

The top status bar shows contextual information:
- `Chains(N)` - Number of configured chains (from `.github/lazydispatch.yml`)
- `Live(N)` or `Live(N*)` - Watched runs count (`*` indicates active runs)
- `Chain: name (step/total)` - Current chain execution progress

## Tabbed Right Panel

Three tabs accessible with `h`/`l` when focused:
- **[History]** - Recent runs for selected workflow with branch, inputs, time
- **Chains** - Configured workflow chains with step counts
- **Live** - Active watched runs with status icons

### Tab Navigation
| Key | Action |
|-----|--------|
| `h` / `Left` | Previous tab |
| `l` / `Right` | Next tab |
| `j` / `k` | Navigate within tab |
| `Enter` | Select item |
| `d` | Clear run (Live tab) |
| `D` | Clear all completed (Live tab) |

### Live Runs Status Icons
| Icon | Status |
|------|--------|
| `o` | Queued |
| `*` | In progress |
| `+` | Success |
| `x` | Failure |
| `-` | Cancelled |

## Configuration Panel

### Header
```
Branch: [b] feature/xyz    Watch: [w] off    [r] reset all
Filter: /env                                  (shown when active)
```

### Input Table
| # | Req | Name | Value | Default |
|---|-----|------|-------|---------|
| 1 | x | environment | production | dev |
| 2 |   | debug | true | false |
| 3 | x | version | dev | dev |  *(dimmed - at default)*
| 4 |   | tag | ("") | ("") |  *(italic - empty string)*

- Numbers 1-9, then 0 for 10th input; no numbers beyond
- `x` in Req column for required inputs
- Rows dimmed when value equals default
- Special values `("")` and `(null)` shown in italics
- `>` indicator for selected row
- Scroll indicators `^`/`v` when content overflows

### CLI Preview
```
Command:
...gh workflow run deploy.yml --ref main -f environment=production [c]
```
Right-justified, truncated with `...` prefix if too long.

## Left Pane Mode Indicator

The left pane title changes based on context:
- **Workflows** - Default workflow list view
- **Workflows > Input** - Viewing input details
- **Workflows > Preview** - Previewing history entry

## Input Details Panel

When navigating inputs with `j/k`, the workflow pane transforms to show:
- Input name and required status
- Type and options (for choice inputs)
- Full description
- Current value and default

Press `Esc` to return to workflow list.

## Keyboard Shortcuts

### Navigation
| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Switch panes |
| `h` / `l` | Switch tabs (right panel) |
| `j/k` or `Up/Down` | Navigate / Select input |
| `Enter` | Execute workflow or edit selected input |
| `Space` | Select workflow and jump to config |
| `Esc` | Deselect input / Close modal |

### Config Panel
| Key | Action |
|-----|--------|
| `1-9`, `0` | Edit input by number |
| `b` | Select branch |
| `w` | Toggle watch mode |
| `/` | Filter inputs (fuzzy match) |
| `c` | Copy command to clipboard |
| `r` | Reset all inputs to defaults |

### Input Editing
| Key | Action |
|-----|--------|
| `Ctrl+R` | Restore default value |
| `Enter` | Confirm (or apply anyway if validation fails) |
| `Esc` | Cancel or keep editing |

## Modals

### Filter Modal
- Fuzzy search on input names
- Live match preview
- Filtered inputs remapped to 1-9, 0

### Reset Modal
- Shows diff of modified values: `current -> default`
- Confirm with `Enter/y`, cancel with `Esc/n`

### Edit Modals
- Description shown at top
- Options listed for choice type
- Default value shown below input
- Validation errors with "apply anyway" option

### Chain Status Modal
- Shows current step and total steps
- Displays workflow name for each step
- Status indicator for each step (pending, running, success, failed)
