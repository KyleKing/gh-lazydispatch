# gh-wfd Extension Plan

## Research Summary

### Existing Solutions

| Tool | Capabilities | Gaps |
|------|--------------|------|
| [`gh workflow run`](https://cli.github.com/manual/gh_workflow_run) (built-in) | Interactive workflow selection, prompts for inputs | No fuzzy filtering, basic input handling, no local YAML parsing |
| [`gh-dispatch`](https://github.com/mdb/gh-dispatch) | Trigger dispatch events, watch runs | No interactive selection, requires JSON input, no fuzzy filtering |
| [`gh-f`](https://github.com/gennaro-tedesco/gh-f) | Fuzzy workflow run viewing/filtering | Views runs only, doesn't trigger new runs or handle inputs |

**Gap identified**: No existing extension provides fuzzy workflow selection + interactive input configuration + branch selection in a single cohesive experience.

### Extension Development Options

| Approach | Pros | Cons |
|----------|------|------|
| **Shell + fzf** | Simple, portable, fast to develop | Requires fzf dependency, limited input handling |
| **Go + go-gh + bubbletea** | Native TUI, no external deps, rich interactions | More complex, longer development time |

**Recommendation**: Go-based extension using:
- [`go-gh/v2`](https://pkg.go.dev/github.com/cli/go-gh/v2) - API access, auth, repository detection
- [`charmbracelet/huh`](https://github.com/charmbracelet/huh) - Forms with Select/Input fields
- [`charmbracelet/bubbles/list`](https://pkg.go.dev/github.com/charmbracelet/bubbles/list) - Fuzzy filterable list selection

---

## Feature Specification

### Core Flow

```
gh wfd
     │
     ▼
┌─────────────────────────────────────┐
│  1. Scan .github/workflows/*.y*ml   │
│     (local filesystem)              │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│  2. Fuzzy-select workflow           │
│     - Type to filter                │
│     - Shows filename + name field   │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│  3. Parse workflow_dispatch inputs  │
│     from selected YAML              │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│  4. Interactive input configuration │
│     - text → Input field            │
│     - boolean → Confirm field       │
│     - choice → Select field         │
│     - Shows defaults/descriptions   │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│  5. Branch selection                │
│     - <current branch> (first)      │
│     - <default branch> (second)     │
│     - Recent branches (sorted)      │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│  6. Confirm and trigger             │
│     gh workflow run <file>          │
│       --ref <branch>                │
│       -f key=value ...              │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│  7. (Optional) Watch run            │
│     gh run watch                    │
└─────────────────────────────────────┘
```

### Input Type Mapping

| `workflow_dispatch` input type | TUI Component |
|-------------------------------|---------------|
| `string` (default) | `huh.NewInput()` |
| `boolean` | `huh.NewConfirm()` |
| `choice` | `huh.NewSelect()` with options |
| `environment` | `huh.NewSelect()` with repo environments |

---

## Implementation Steps

### Phase 1: Project Setup
1. Initialize Go extension: `gh extension create --precompiled=go gh-wfd`
2. Add dependencies: go-gh/v2, charmbracelet/huh, gopkg.in/yaml.v3
3. Set up basic CLI structure with cobra or minimal main.go

### Phase 2: Workflow Discovery
1. Detect git root directory
2. Glob for `.github/workflows/*.yml` and `.github/workflows/*.yaml`
3. Parse each file to extract:
   - `name` field (display name)
   - `on.workflow_dispatch` presence (filter to only dispatchable workflows)
   - `on.workflow_dispatch.inputs` schema

### Phase 3: Workflow Selection
1. Build list of dispatchable workflows
2. Implement fuzzy-filterable list using `bubbles/list`
3. Display: `workflow-name (filename.yml)`

### Phase 4: Input Configuration
1. For selected workflow, read input definitions
2. Build dynamic `huh.Form` based on input types:
   - Title from input key
   - Description from input description
   - Default value from input default
   - Options from input options (for choice type)
3. Run form and collect values

### Phase 5: Branch Selection
1. Get current branch: `git branch --show-current`
2. Get default branch: `gh repo view --json defaultBranchRef`
3. Get recent branches: `git branch --sort=-committerdate`
4. Present as Select with ordering:
   - Current branch (marked)
   - Default branch (marked)
   - Other recent branches

### Phase 6: Execution
1. Build `gh workflow run` command with collected inputs
2. Execute via `go-gh.Exec()` or `go-gh.ExecInteractive()`
3. Optionally prompt to watch: `gh run watch`

### Phase 7: Polish
1. Add `--help` documentation
2. Handle edge cases (no workflows, no inputs, errors)
3. Add `--dry-run` flag to preview command
4. Cross-compile for releases using `gh-extension-precompile` action

---

## File Structure

```
gh-wfd/
├── main.go                 # Entry point, CLI setup
├── internal/
│   ├── workflow/
│   │   ├── discovery.go    # Find and parse workflow files
│   │   └── types.go        # Workflow/Input struct definitions
│   ├── ui/
│   │   ├── selector.go     # Fuzzy workflow selection
│   │   ├── inputs.go       # Dynamic input form builder
│   │   └── branch.go       # Branch selection
│   └── runner/
│       └── execute.go      # gh workflow run invocation
├── go.mod
├── go.sum
└── .github/
    └── workflows/
        └── release.yml     # gh-extension-precompile action
```

---

## Dependencies

```go
require (
    github.com/cli/go-gh/v2 v2.13.0
    github.com/charmbracelet/huh v0.6.0
    github.com/charmbracelet/bubbles v0.20.0
    github.com/charmbracelet/bubbletea v1.2.0
    gopkg.in/yaml.v3 v3.0.1
)
```

---

## Alternative: Shell + fzf (Simpler Approach)

If a faster MVP is preferred, a shell script approach:

```bash
#!/bin/bash
# gh-wfd

WORKFLOW=$(find .github/workflows -name "*.y*ml" | fzf --preview 'cat {}')
# Parse inputs with yq
# Build gh workflow run command
# Execute
```

**Tradeoffs**:
- Pros: Fast to build, leverages existing tools
- Cons: Requires fzf + yq, less portable, harder to handle complex input types

---

---

## YAML Parsing Complexity

### Difficulty: Low-Medium

The `workflow_dispatch` schema is well-defined with limited nesting. Parsing requires:

```go
type WorkflowFile struct {
    Name string `yaml:"name"`
    On   struct {
        WorkflowDispatch *struct {
            Inputs map[string]WorkflowInput `yaml:"inputs"`
        } `yaml:"workflow_dispatch"`
    } `yaml:"on"`
}

type WorkflowInput struct {
    Description string   `yaml:"description"`
    Required    bool     `yaml:"required"`
    Default     string   `yaml:"default"`
    Type        string   `yaml:"type"`    // string|boolean|choice|number|environment
    Options     []string `yaml:"options"` // for choice type
}
```

### Edge Cases to Handle

1. **`on` can be a string or map**: `on: push` vs `on: { workflow_dispatch: ... }`
2. **Boolean quirk**: GitHub converts booleans to strings in `github.event.inputs`, but preserves them in `inputs` context
3. **Missing type**: Defaults to `string` when `type` is omitted
4. **Environment type**: Requires API call to fetch repo environments

### gopkg.in/yaml.v3 handles this cleanly

```go
var wf WorkflowFile
if err := yaml.Unmarshal(data, &wf); err != nil {
    return err
}
if wf.On.WorkflowDispatch == nil {
    // Not a dispatchable workflow
}
```

---

## TUI Library Comparison

### Option A: charmbracelet/huh (Recommended for forms)

**What to expect**:
- Pre-built Input, Select, Confirm, MultiSelect fields
- Built-in validation, descriptions, theming
- Accessible mode for screen readers
- Keyboard: arrow keys navigate, enter confirms, esc cancels

```go
form := huh.NewForm(
    huh.NewGroup(
        huh.NewSelect[string]().
            Title("Log Level").
            Description("Set verbosity").
            Options(
                huh.NewOption("info", "info"),
                huh.NewOption("warning", "warning").Selected(true),
                huh.NewOption("debug", "debug"),
            ).
            Value(&logLevel),
        huh.NewConfirm().
            Title("Dry run?").
            Value(&dryRun),
    ),
)
form.Run()
```

### Option B: charmbracelet/bubbles/list (Recommended for workflow selection)

**What to expect**:
- Fuzzy filtering via [sahilm/fuzzy](https://github.com/sahilm/fuzzy)
- Pagination, spinner, status messages
- Type to filter, arrow keys to navigate

```go
items := []list.Item{
    workflowItem{name: "CI", file: "ci.yml"},
    workflowItem{name: "Deploy", file: "deploy.yml"},
}
l := list.New(items, list.NewDefaultDelegate(), 0, 0)
l.Title = "Select Workflow"
// Fuzzy filtering is enabled by default
```

### Option C: Full bubbletea TUI (Most flexible, most work)

**When to use**: If you want a single-screen dashboard showing workflow + inputs + branch simultaneously.

**Elm Architecture**:
- Model: struct holding all state
- Update: receives keypresses/events, returns new model
- View: renders model to string

---

## UX Design: Fast & Intuitive

### Design Goals

| Goal | Solution |
|------|----------|
| Minimal keystrokes | Frecency-sorted defaults, smart pre-selection |
| Discoverability | Inline help, descriptions from YAML |
| Speed | Local YAML parsing (no API for inputs) |
| Repeatability | History with "re-run last" shortcut |

### Interaction Flow (Optimized)

```
┌────────────────────────────────────────────────────────┐
│  gh wfd                                    │
│                                                        │
│  Recent:                                               │
│  > [1] deploy.yml → production (main) ← Enter to run  │
│    [2] ci.yml → staging (feature-x)                   │
│    [3] release.yml → v1.2.0 (main)                    │
│                                                        │
│  ─────────────────────────────────────────────────────│
│  All workflows: (type to filter)                       │
│    ci.yml                                              │
│    deploy.yml                                          │
│    release.yml                                         │
│    test.yml                                            │
│                                                        │
│  [↑↓] navigate  [enter] select  [1-3] quick-run       │
└────────────────────────────────────────────────────────┘
```

### Quick Actions

| Key | Action |
|-----|--------|
| `1-9` | Re-run recent workflow with same inputs |
| `Enter` | Select highlighted item |
| `Tab` | Skip to next field / use default |
| `Ctrl+R` | Toggle "watch run after trigger" |

---

## Frecency & History Storage

### Storage Location

Follow XDG Base Directory spec via [go-xdg](https://pkg.go.dev/launchpad.net/go-xdg):

```
~/.local/share/gh-wfd/
├── history.json       # Recent runs with inputs
└── frecency.db        # SQLite for scoring (optional)
```

### Simple JSON History (MVP)

```go
type HistoryEntry struct {
    Repo       string            `json:"repo"`
    Workflow   string            `json:"workflow"`
    Branch     string            `json:"branch"`
    Inputs     map[string]string `json:"inputs"`
    RunCount   int               `json:"run_count"`
    LastRunAt  time.Time         `json:"last_run_at"`
}
```

### Frecency Algorithm

Score = (frequency weight) × (recency weight)

```go
func frecencyScore(entry HistoryEntry) float64 {
    hoursSince := time.Since(entry.LastRunAt).Hours()
    recency := 1.0
    switch {
    case hoursSince < 1:
        recency = 4.0
    case hoursSince < 24:
        recency = 2.0
    case hoursSince < 168: // 1 week
        recency = 1.0
    default:
        recency = 0.5
    }
    return float64(entry.RunCount) * recency
}
```

### SQLite (For Power Users)

If history grows large, [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) provides:
- Efficient queries for top-N by frecency
- Per-repo history isolation
- Note: CGO required, increases binary size

---

## Full TUI vs Sequential Prompts

### Sequential Prompts (Simpler, Recommended for v1)

```
$ gh wfd

? Select workflow: (type to filter)
  > deploy.yml (Deploy to Environment)
    ci.yml (CI Pipeline)
    release.yml (Create Release)

? Environment: (type: choice)
  > production
    staging
    development

? Dry run: (type: boolean)
  > Yes
    No

? Branch:
  > feature-xyz (current)
    main (default)
    develop

Triggering: gh workflow run deploy.yml --ref feature-xyz -f environment=production -f dry_run=true
Watch run? [Y/n]
```

### Full TUI Dashboard (v2 consideration)

Single screen with split panes:
- Left: workflow list with fuzzy filter
- Right: input form for selected workflow
- Bottom: branch selector + action buttons

Requires more bubbletea wiring but provides:
- See inputs before committing to workflow
- Faster switching between workflows
- Visual confirmation of all settings

---

## Recommended MVP Scope

### Include in v1

1. Fuzzy workflow selection (bubbles/list)
2. Dynamic input forms (huh)
3. Branch selection with current/default priority
4. JSON history file with frecency sorting
5. `--dry-run` flag
6. `--watch` flag to tail run

### Defer to v2

1. Full TUI dashboard
2. SQLite history
3. Environment type (requires API)
4. Number input validation
5. Config file for defaults

---

## References

- [Creating GitHub CLI extensions](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions)
- [go-gh library](https://pkg.go.dev/github.com/cli/go-gh/v2)
- [charmbracelet/huh forms](https://github.com/charmbracelet/huh)
- [charmbracelet/bubbles list](https://pkg.go.dev/github.com/charmbracelet/bubbles/list)
- [gh workflow run manual](https://cli.github.com/manual/gh_workflow_run)
- [workflow_dispatch input types](https://github.blog/changelog/2021-11-10-github-actions-input-types-for-manual-workflows/)
- [Atuin shell history](https://atuin.sh/) - frecency inspiration
- [gh-extension-precompile action](https://github.com/cli/gh-extension-precompile)
