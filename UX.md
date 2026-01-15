# Configuration Panel UX

## Layout

Three-pane layout:
- **Top-left**: Workflow list (or Input Details when an input is selected)
- **Top-right**: Run history for selected workflow
- **Bottom**: Configuration panel with table-based input display

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
| `j/k` or `↑/↓` | Navigate / Select input |
| `Enter` | Execute workflow or edit selected input |
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
| `Alt+D` | Restore default value |
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
