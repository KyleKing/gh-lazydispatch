## Innovative Log Viewer Features

This document proposes innovative features for the log viewer that leverage the existing architecture and enhance the UX.

## Core Features (Implemented)

### ✓ Multi-Step Tabs
- **What:** Navigate between logs from different workflow steps
- **Why:** Maintains step separation while allowing quick comparison
- **UX:** Tab key cycles through steps, visual indicator shows active step

### ✓ Smart Filtering
- **What:** Pre-defined filters (all, errors, warnings) + custom search
- **Why:** Quickly focus on relevant log entries
- **UX:** 'f' key cycles filters, '/' key opens search, 'n/N' navigate matches

### ✓ Log Caching
- **What:** Store fetched logs locally with TTL
- **Why:** Instant access to previously viewed logs
- **UX:** Transparent - faster load times for repeat access

### ✓ Error-First Mode
- **What:** When opened from error modal, pre-filter to errors
- **Why:** Immediately show what failed without manual filtering
- **UX:** Automatic - no user action needed

## Innovative Features (Proposed)

### 1. **Log Diff Mode** 🆕

**Concept:** Compare logs from two different chain executions side-by-side

**Use Cases:**
- "Why did this run fail when the previous one succeeded?"
- "What changed between the staging and production deployment?"
- "Compare output before and after code change"

**UX Design:**
```
┌─────────────────────────────────────────────────────────────┐
│  Comparing: release-bump (Run #123 vs Run #145)             │
├──────────────────────────┬──────────────────────────────────┤
│  Step 1: ci-gate.yml     │  Step 1: ci-gate.yml            │
│  ✓ All checks passed     │  ✗ tests failed                 │
│                          │  Error: 3 tests failed          │
│  ← SAME                  │                                  │
│                          │  → DIFFERENT                     │
├──────────────────────────┼──────────────────────────────────┤
│  Step 2: version-bump    │  Step 2: version-bump           │
│  ✓ Bumped to v1.2.3      │  - Skipped (previous failed)    │
└──────────────────────────┴──────────────────────────────────┘
[d] toggle diff  [=] align scrolling  [tab] next step
```

**Implementation:**
```go
type LogDiffMode struct {
	leftRun  *logs.RunLogs
	rightRun *logs.RunLogs
	aligned  bool // sync scrolling
}

func (m *LogsViewerModal) EnableDiffMode(otherRun *logs.RunLogs) {
	m.diffMode = &LogDiffMode{
		leftRun:  m.runLogs,
		rightRun: otherRun,
		aligned:  true,
	}
	m.renderDiff()
}
```

### 2. **Smart Log Summarization** 🆕

**Concept:** AI/heuristic-based summary of key events in logs

**Use Cases:**
- "Give me the TL;DR of this 10,000 line log"
- "What were the key milestones in this deployment?"
- "Extract all error messages"

**UX Design:**
```
┌─────────────────────────────────────────────────────────────┐
│  Summary: ci-gate.yml (12,453 lines)                         │
├─────────────────────────────────────────────────────────────┤
│  ⏱  Duration: 8m 23s                                        │
│  ✓  23 tests passed                                          │
│  ✗  3 tests failed:                                          │
│     → test_user_authentication (auth.test.ts:45)            │
│     → test_api_timeout (api.test.ts:120)                    │
│     → test_database_migration (db.test.ts:89)               │
│  ⚠  5 warnings:                                              │
│     → Deprecated API usage (2 occurrences)                  │
│     → Slow query detected (3 occurrences)                   │
│  📊 Resource usage:                                          │
│     → Peak memory: 2.3 GB                                   │
│     → CPU time: 4m 12s                                      │
└─────────────────────────────────────────────────────────────┘
[enter] view full logs  [s] toggle summary
```

**Implementation:**
```go
type LogSummary struct {
	Duration      time.Duration
	TestsPassed   int
	TestsFailed   int
	Warnings      []string
	Errors        []string
	KeyMilestones []Milestone
	ResourceUsage *ResourceStats
}

func GenerateSummary(stepLogs *logs.StepLogs) *LogSummary {
	// Parse logs using regex patterns
	summary := &LogSummary{}

	for _, entry := range stepLogs.Entries {
		// Detect test results
		if testPattern.MatchString(entry.Content) {
			// Extract test name and result
		}

		// Detect duration markers
		if durationPattern.MatchString(entry.Content) {
			// Extract timing information
		}

		// Collect errors and warnings
		if entry.Level == logs.LogLevelError {
			summary.Errors = append(summary.Errors, entry.Content)
		}
	}

	return summary
}
```

### 3. **Log Bookmarks & Annotations** 🆕

**Concept:** Mark interesting log lines for later review

**Use Cases:**
- "Remember this suspicious warning for later investigation"
- "Bookmark where the error first appeared"
- "Annotate this section with context"

**UX Design:**
```
┌─────────────────────────────────────────────────────────────┐
│  Step 2: version-bump.yml                                    │
├─────────────────────────────────────────────────────────────┤
│  [12:34:05] Starting version bump...                         │
│  [12:34:06] Reading version from package.json                │
│ 🔖 [12:34:07] Warning: No conventional commits found        │
│  │  Note: This happened because we squash-merged the PR     │
│  [12:34:08] Falling back to patch bump                       │
│ 🔖 [12:34:10] Error: Permission denied writing to file      │
│  │  TODO: Check GitHub token permissions                    │
│  [12:34:11] Retrying with elevated permissions...           │
└─────────────────────────────────────────────────────────────┘
[m] add bookmark  [e] edit note  [j/k] next/prev bookmark
```

**Implementation:**
```go
type LogBookmark struct {
	StepIndex int
	LineIndex int
	Note      string
	CreatedAt time.Time
	Tags      []string
}

type BookmarkStore struct {
	bookmarks map[string][]LogBookmark // keyed by chainName:runID
	mu        sync.RWMutex
}

func (m *LogsViewerModal) AddBookmark() {
	bookmark := LogBookmark{
		StepIndex: m.activeTab,
		LineIndex: m.viewport.YOffset,
		CreatedAt: time.Now(),
	}
	m.bookmarks = append(m.bookmarks, bookmark)
	m.persistBookmarks()
}
```

### 4. **Timeline View** 🆕

**Concept:** Visual timeline of events across all steps

**Use Cases:**
- "When did each step start and end?"
- "Identify bottlenecks in the execution"
- "See parallel vs sequential execution"

**UX Design:**
```
┌─────────────────────────────────────────────────────────────┐
│  Timeline: release-bump                                      │
├─────────────────────────────────────────────────────────────┤
│  12:30                12:35                12:40      12:42  │
│  │                     │                   │          │      │
│  ├─────────────────────┤ ci-gate (5m 23s)                   │
│  │                     ✓                                     │
│                        ├──────────────────┤ version-bump     │
│                        │                  ✓ (4m 12s)        │
│                                                              │
│  Events:                                                     │
│  12:30:05  Started ci-gate.yml                              │
│  12:32:18  Tests completed (98 passed)                      │
│  12:35:28  CI gate passed ✓                                 │
│  12:35:30  Started version-bump.yml                         │
│  12:39:42  Version bumped to v1.2.3                         │
│  12:39:42  Completed successfully ✓                         │
└─────────────────────────────────────────────────────────────┘
[t] toggle timeline  [z] zoom in/out
```

**Implementation:**
```go
type TimelineEvent struct {
	Timestamp   time.Time
	StepIndex   int
	EventType   string // started, completed, error, warning
	Description string
	Duration    time.Duration
}

func GenerateTimeline(runLogs *logs.RunLogs) []TimelineEvent {
	var events []TimelineEvent

	for _, step := range runLogs.AllSteps() {
		// Extract timestamps from logs
		startTime, endTime := extractStepTimes(step)

		events = append(events, TimelineEvent{
			Timestamp:   startTime,
			StepIndex:   step.StepIndex,
			EventType:   "started",
			Description: fmt.Sprintf("Started %s", step.Workflow),
		})

		// Add milestone events
		for _, entry := range step.Entries {
			if isImportantEvent(entry) {
				events = append(events, TimelineEvent{
					Timestamp:   entry.Timestamp,
					StepIndex:   step.StepIndex,
					EventType:   string(entry.Level),
					Description: entry.Content,
				})
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events
}
```

### 5. **Log Export with Context** 🆕

**Concept:** Export filtered logs with rich context and formatting

**Use Cases:**
- "Share these specific error logs with the team"
- "Create a bug report with relevant logs"
- "Archive logs for compliance"

**Formats:**
- **Markdown:** With syntax highlighting and links
- **HTML:** Styled, searchable, self-contained
- **JSON:** Structured data with metadata
- **Text:** Plain text with optional ANSI colors

**UX Design:**
```
┌─────────────────────────────────────────────────────────────┐
│  Export Logs                                                 │
├─────────────────────────────────────────────────────────────┤
│  Format:                                                     │
│    ◉ Markdown (GitHub-flavored)                             │
│    ○ HTML (self-contained)                                  │
│    ○ JSON (structured)                                      │
│    ○ Plain text                                             │
│                                                              │
│  Include:                                                    │
│    ☑ Current filter (errors only)                           │
│    ☑ Timestamps                                             │
│    ☑ Step names                                             │
│    ☑ Search highlights                                      │
│    ☑ Workflow URLs                                          │
│    ☐ Full metadata                                          │
│                                                              │
│  Output: ~/Downloads/release-bump-errors-2026-01-19.md     │
└─────────────────────────────────────────────────────────────┘
[enter] export  [esc] cancel
```

**Implementation:**
```go
type ExportConfig struct {
	Format           string // markdown, html, json, text
	IncludeTimestamp bool
	IncludeStepNames bool
	IncludeHighlight bool
	IncludeMetadata  bool
	OutputPath       string
}

func ExportLogs(filtered *logs.FilteredResult, config ExportConfig) error {
	switch config.Format {
	case "markdown":
		return exportMarkdown(filtered, config)
	case "html":
		return exportHTML(filtered, config)
	case "json":
		return exportJSON(filtered, config)
	default:
		return exportText(filtered, config)
	}
}

func exportMarkdown(filtered *logs.FilteredResult, config ExportConfig) error {
	var md strings.Builder

	md.WriteString("# Logs Export\n\n")
	md.WriteString(fmt.Sprintf("**Chain:** %s\n", filtered.ChainName))
	md.WriteString(fmt.Sprintf("**Filter:** %s\n\n", filtered.Config.Level))

	for _, step := range filtered.Steps {
		md.WriteString(fmt.Sprintf("## Step %d: %s\n\n", step.StepIndex+1, step.StepName))
		md.WriteString("```\n")

		for _, entry := range step.Entries {
			if config.IncludeTimestamp {
				md.WriteString(entry.Original.Timestamp.Format("15:04:05") + " ")
			}
			md.WriteString(entry.Original.Content)
			md.WriteString("\n")
		}

		md.WriteString("```\n\n")
	}

	return os.WriteFile(config.OutputPath, []byte(md.String()), 0644)
}
```

### 6. **Log Streaming with Live Update** 🆕

**Concept:** Watch logs update in real-time as chain executes

**Use Cases:**
- "Monitor long-running deployment"
- "Catch errors as they happen"
- "See progress without waiting for completion"

**UX Design:**
```
┌─────────────────────────────────────────────────────────────┐
│  Live: ci-gate.yml (running)                         ⚡ LIVE │
├─────────────────────────────────────────────────────────────┤
│  [12:45:12] Setting up job...                                │
│  [12:45:15] Installing dependencies...                       │
│  [12:45:42] Running tests...                                 │
│  [12:46:01] ✓ test_user_login                               │
│  [12:46:02] ✓ test_user_signup                              │
│  [12:46:03] ⏳ test_api_endpoint (running...)               │
│  │                                                           │
│  └─ Auto-scrolling (press 's' to stop)                      │
└─────────────────────────────────────────────────────────────┘
[s] stop auto-scroll  [p] pause stream  [q] detach and close
```

**Implementation:**
```go
type StreamState struct {
	Active      bool
	AutoScroll  bool
	Paused      bool
	LastFetched time.Time
}

func (m *LogsViewerModal) EnableStreaming(runID int64) tea.Cmd {
	return func() tea.Msg {
		// Start polling for new log entries
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if !m.streamState.Paused {
					newEntries := fetchNewLogsSince(runID, m.streamState.LastFetched)
					if len(newEntries) > 0 {
						return LogStreamUpdateMsg{
							Entries: newEntries,
						}
					}
				}
			}
		}
	}
}

// In Update():
case LogStreamUpdateMsg:
	// Append new entries
	currentStep := m.filtered.Steps[m.activeTab]
	for _, entry := range msg.Entries {
		currentStep.Entries = append(currentStep.Entries, entry)
	}

	// Auto-scroll to bottom if enabled
	if m.streamState.AutoScroll {
		m.viewport.GotoBottom()
	}

	m.updateViewportContent()
	return m, m.EnableStreaming(m.currentRunID)
```

### 7. **Log Pattern Detection** 🆕

**Concept:** Automatically detect common patterns and anomalies

**Patterns to Detect:**
- **Timeouts:** "timed out", "timeout exceeded"
- **Memory issues:** "out of memory", "heap overflow"
- **Network errors:** "connection refused", "DNS lookup failed"
- **Permission errors:** "permission denied", "access forbidden"
- **Retry patterns:** Multiple attempts, exponential backoff
- **Performance degradation:** Increasing response times

**UX Design:**
```
┌─────────────────────────────────────────────────────────────┐
│  Detected Patterns:                                          │
├─────────────────────────────────────────────────────────────┤
│  ⚠  Timeout Pattern (3 occurrences)                         │
│     Lines: 145, 289, 432                                    │
│     Suggestion: Check network connectivity or increase       │
│                 timeout threshold                            │
│                                                              │
│  ⚠  Retry Loop (5 attempts)                                 │
│     Lines: 510-555                                          │
│     Suggestion: Operation failed after retries, check        │
│                 upstream service status                      │
│                                                              │
│  ℹ  Performance Note                                        │
│     API response time increased from 120ms to 850ms         │
│     Lines: 100-400                                          │
└─────────────────────────────────────────────────────────────┘
[p] view pattern details  [j/k] next/prev pattern
```

**Implementation:**
```go
type LogPattern struct {
	Name        string
	Description string
	Severity    string // info, warning, error
	LineNumbers []int
	Suggestion  string
}

var CommonPatterns = []PatternMatcher{
	{
		Name:    "Timeout",
		Regex:   regexp.MustCompile(`(?i)(timeout|timed out)`),
		Severity: "warning",
		Suggestion: "Check network connectivity or increase timeout threshold",
	},
	{
		Name:    "Out of Memory",
		Regex:   regexp.MustCompile(`(?i)(out of memory|oom|heap overflow)`),
		Severity: "error",
		Suggestion: "Increase memory allocation or optimize memory usage",
	},
	// ... more patterns
}

func DetectPatterns(stepLogs *logs.StepLogs) []LogPattern {
	var detected []LogPattern

	for _, matcher := range CommonPatterns {
		var matches []int
		for i, entry := range stepLogs.Entries {
			if matcher.Regex.MatchString(entry.Content) {
				matches = append(matches, i)
			}
		}

		if len(matches) > 0 {
			detected = append(detected, LogPattern{
				Name:        matcher.Name,
				Description: matcher.Description,
				Severity:    matcher.Severity,
				LineNumbers: matches,
				Suggestion:  matcher.Suggestion,
			})
		}
	}

	return detected
}
```

### 8. **Contextual Log Navigation** 🆕

**Concept:** Smart navigation based on log structure

**Features:**
- **Jump to error:** Find first/last error in logs
- **Jump to test:** Navigate by test name
- **Jump to section:** Common log sections (setup, build, test, deploy)
- **Jump to timestamp:** Go to specific time

**UX Design:**
```
┌─────────────────────────────────────────────────────────────┐
│  Quick Navigation                                            │
├─────────────────────────────────────────────────────────────┤
│  [1] First error       (line 145)                           │
│  [2] Last error        (line 892)                           │
│  [3] Setup phase       (line 10)                            │
│  [4] Build phase       (line 120)                           │
│  [5] Test phase        (line 450)                           │
│  [6] Deploy phase      (line 800)                           │
│  [t] Jump to time...                                        │
└─────────────────────────────────────────────────────────────┘
```

**Implementation:**
```go
type LogSection struct {
	Name      string
	StartLine int
	EndLine   int
	Type      string // setup, build, test, deploy, cleanup
}

func DetectSections(stepLogs *logs.StepLogs) []LogSection {
	var sections []LogSection

	sectionMarkers := map[string]*regexp.Regexp{
		"setup":  regexp.MustCompile(`(?i)(setup|initialize|preparing)`),
		"build":  regexp.MustCompile(`(?i)(build|compil|bundl)`),
		"test":   regexp.MustCompile(`(?i)(test|spec|check)`),
		"deploy": regexp.MustCompile(`(?i)(deploy|publish|release)`),
	}

	currentSection := ""
	startLine := 0

	for i, entry := range stepLogs.Entries {
		for sectionType, pattern := range sectionMarkers {
			if pattern.MatchString(entry.Content) {
				// Close previous section
				if currentSection != "" {
					sections = append(sections, LogSection{
						Name:      currentSection,
						StartLine: startLine,
						EndLine:   i - 1,
						Type:      currentSection,
					})
				}

				// Start new section
				currentSection = sectionType
				startLine = i
				break
			}
		}
	}

	return sections
}

func (m *LogsViewerModal) ShowQuickNav() {
	sections := DetectSections(m.currentStepLogs)
	errors := findErrors(m.currentStepLogs)

	m.quickNavItems = []NavItem{
		{Label: "First error", Line: errors[0].LineIndex},
		{Label: "Last error", Line: errors[len(errors)-1].LineIndex},
	}

	for _, section := range sections {
		m.quickNavItems = append(m.quickNavItems, NavItem{
			Label: section.Name + " phase",
			Line:  section.StartLine,
		})
	}

	m.showingQuickNav = true
}
```

### 9. **Log Collaboration Features** 🆕

**Concept:** Share and discuss logs with team members

**Features:**
- **Permalink generation:** Create shareable links to specific log lines
- **Comment threads:** Discuss specific log entries
- **Share filtered view:** Send link with filters pre-applied

**UX Design:**
```
┌─────────────────────────────────────────────────────────────┐
│  Share Logs                                                  │
├─────────────────────────────────────────────────────────────┤
│  Shareable URL:                                             │
│  lazydispatch://logs/release-bump/12345?                    │
│    step=1&                                                  │
│    filter=errors&                                           │
│    highlight=timeout&                                       │
│    line=145                                                 │
│                                                              │
│  📋 [Copy URL]  🌐 [Open in GitHub]                        │
│                                                              │
│  Or export and attach to:                                   │
│  • GitHub issue                                             │
│  • Slack thread                                             │
│  • Email                                                    │
└─────────────────────────────────────────────────────────────┘
```

**Implementation:**
```go
type ShareableLogLink struct {
	ChainName  string
	RunID      int64
	StepIndex  int
	FilterType logs.FilterLevel
	SearchTerm string
	Line       int
}

func GenerateShareableURL(link ShareableLogLink) string {
	return fmt.Sprintf(
		"lazydispatch://logs/%s/%d?step=%d&filter=%s&highlight=%s&line=%d",
		link.ChainName,
		link.RunID,
		link.StepIndex,
		link.FilterType,
		url.QueryEscape(link.SearchTerm),
		link.Line,
	)
}

// Handle URL scheme
func HandleShareableURL(url string) tea.Cmd {
	// Parse URL parameters
	// Fetch logs
	// Open viewer with specified filters and position
}
```

### 10. **Performance Profiling View** 🆕

**Concept:** Extract and visualize performance metrics from logs

**Metrics to Track:**
- Test execution times
- API response times
- Database query durations
- Build times
- Memory usage over time

**UX Design:**
```
┌─────────────────────────────────────────────────────────────┐
│  Performance Profile: ci-gate.yml                            │
├─────────────────────────────────────────────────────────────┤
│  Test Execution Times:                                       │
│                                                              │
│  test_user_login      ▓▓░░░░░░░░ 123ms                      │
│  test_api_timeout     ▓▓▓▓▓▓▓▓▓▓ 850ms ⚠ SLOW              │
│  test_db_query        ▓░░░░░░░░░  89ms                      │
│                                                              │
│  Slowest Operations:                                         │
│  1. API health check          2.3s (line 234)               │
│  2. Database migration        1.8s (line 456)               │
│  3. Asset compilation         1.2s (line 789)               │
│                                                              │
│  Memory Usage:                                               │
│  Peak: 2.3 GB (at 12:42:15)                                 │
│  Average: 1.8 GB                                            │
└─────────────────────────────────────────────────────────────┘
```

**Implementation:**
```go
type PerformanceProfile struct {
	TestTimes       map[string]time.Duration
	SlowOperations  []SlowOperation
	MemoryUsage     []MemorySnapshot
	CPUUsage        []CPUSnapshot
}

type SlowOperation struct {
	Name     string
	Duration time.Duration
	Line     int
}

func ExtractPerformanceMetrics(stepLogs *logs.StepLogs) *PerformanceProfile {
	profile := &PerformanceProfile{
		TestTimes:      make(map[string]time.Duration),
		SlowOperations: make([]SlowOperation, 0),
	}

	// Parse test execution times
	testPattern := regexp.MustCompile(`(\w+)\s+\((\d+(?:\.\d+)?)([m]?s)\)`)

	for i, entry := range stepLogs.Entries {
		matches := testPattern.FindStringSubmatch(entry.Content)
		if len(matches) == 4 {
			testName := matches[1]
			duration := parseDuration(matches[2], matches[3])
			profile.TestTimes[testName] = duration

			// Flag slow operations
			if duration > 500*time.Millisecond {
				profile.SlowOperations = append(profile.SlowOperations, SlowOperation{
					Name:     testName,
					Duration: duration,
					Line:     i,
				})
			}
		}
	}

	return profile
}
```

## Implementation Priority

### Phase 1: Core Enhancements (Week 1-2)
1. ✅ Multi-step tabs
2. ✅ Smart filtering
3. ✅ Error-first mode
4. 🔨 Log export (markdown)
5. 🔨 Quick navigation

### Phase 2: Analysis Features (Week 3-4)
6. 🔨 Pattern detection
7. 🔨 Log summarization
8. 🔨 Performance profiling
9. 🔨 Bookmarks

### Phase 3: Advanced Features (Week 5-6)
10. 🔨 Timeline view
11. 🔨 Diff mode
12. 🔨 Log streaming

### Phase 4: Collaboration (Week 7+)
13. 🔨 Shareable links
14. 🔨 Comment threads
15. 🔨 Team integrations

## Architectural Patterns

### Observable Pattern for Real-Time Updates

```go
type LogObserver interface {
	OnNewEntry(entry logs.LogEntry)
	OnError(err error)
	OnComplete()
}

type LogStream struct {
	observers []LogObserver
	mu        sync.RWMutex
}

func (ls *LogStream) Subscribe(observer LogObserver) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.observers = append(ls.observers, observer)
}

func (ls *LogStream) Notify(entry logs.LogEntry) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	for _, observer := range ls.observers {
		observer.OnNewEntry(entry)
	}
}
```

### Command Pattern for Actions

```go
type LogViewerAction interface {
	Execute(*LogsViewerModal) error
	Undo(*LogsViewerModal) error
}

type FilterAction struct {
	previousFilter logs.FilterConfig
	newFilter      logs.FilterConfig
}

func (a *FilterAction) Execute(m *LogsViewerModal) error {
	a.previousFilter = *m.filterCfg
	m.filterCfg = &a.newFilter
	return m.applyFilter()
}

func (a *FilterAction) Undo(m *LogsViewerModal) error {
	m.filterCfg = &a.previousFilter
	return m.applyFilter()
}
```

### Strategy Pattern for Export Formats

```go
type ExportStrategy interface {
	Export(filtered *logs.FilteredResult, writer io.Writer) error
	FileExtension() string
	MIMEType() string
}

type MarkdownExporter struct{}
type HTMLExporter struct{}
type JSONExporter struct{}

func (e *MarkdownExporter) Export(filtered *logs.FilteredResult, w io.Writer) error {
	// Generate markdown
}

type LogExporter struct {
	strategy ExportStrategy
}

func (e *LogExporter) ExportToFile(filtered *logs.FilteredResult, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return e.strategy.Export(filtered, file)
}
```

## Performance Optimizations

### Virtual Scrolling for Large Logs

```go
type VirtualViewport struct {
	totalLines   int
	visibleLines int
	topLine      int
	buffer       []string // Only visible + buffer
}

func (v *VirtualViewport) Render() string {
	// Only render visible portion + small buffer
	start := max(0, v.topLine-10)
	end := min(v.totalLines, v.topLine+v.visibleLines+10)

	return strings.Join(v.buffer[start:end], "\n")
}
```

### Incremental Filtering

```go
type IncrementalFilter struct {
	lastResult *logs.FilteredResult
	lastQuery  string
}

func (f *IncrementalFilter) Apply(query string) *logs.FilteredResult {
	// If new query is extension of previous, filter from lastResult
	if strings.HasPrefix(query, f.lastQuery) {
		return f.filterFromPrevious(query)
	}

	// Otherwise, full filter
	return f.filterFromScratch(query)
}
```

### Lazy Loading of Log Details

```go
type LazyStepLogs struct {
	metadata  *logs.StepLogs
	entries   []logs.LogEntry
	loaded    bool
	loader    func() ([]logs.LogEntry, error)
}

func (l *LazyStepLogs) GetEntries() []logs.LogEntry {
	if !l.loaded {
		entries, _ := l.loader()
		l.entries = entries
		l.loaded = true
	}
	return l.entries
}
```

## Testing Strategies

### Snapshot Testing for UI

```go
func TestLogsViewerModal_Render(t *testing.T) {
	tests := []struct {
		name     string
		runLogs  *logs.RunLogs
		filter   logs.FilterConfig
		expected string
	}{
		{
			name:     "error filter",
			runLogs:  testRunLogs(),
			filter:   logs.FilterConfig{Level: logs.FilterErrors},
			expected: "testdata/error-filter.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modal := NewLogsViewerModal(tt.runLogs, 80, 24)
			got := modal.View()

			golden := filepath.Join("testdata", tt.expected)
			if *update {
				os.WriteFile(golden, []byte(got), 0644)
			}

			want, _ := os.ReadFile(golden)
			assert.Equal(t, string(want), got)
		})
	}
}
```

### Property-Based Testing for Filters

```go
func TestFilter_Properties(t *testing.T) {
	// Property: Filtering twice should equal filtering once
	rapid.Check(t, func(t *rapid.T) {
		runLogs := generateRandomRunLogs(t)
		filter := generateRandomFilter(t)

		result1 := filter.Apply(runLogs)
		result2 := filter.Apply(runLogsFromResult(result1))

		assert.Equal(t, result1, result2)
	})
}
```

## Accessibility Considerations

1. **Keyboard Navigation:** All features accessible via keyboard
2. **Screen Reader Support:** Meaningful labels and descriptions
3. **High Contrast Mode:** Respect terminal color preferences
4. **Configurable Keybindings:** Allow customization
5. **Text-only Mode:** Fallback when icons unavailable

## Next Steps

1. Implement core log viewer with tabs and filtering
2. Add export functionality (markdown first)
3. Integrate with chain execution flow
4. Add pattern detection for common issues
5. Implement timeline view
6. Add diff mode for comparing runs
7. Build real-time streaming support
8. Add collaboration features
