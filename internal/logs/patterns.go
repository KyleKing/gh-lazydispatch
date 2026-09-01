package logs

import "regexp"

// Pattern is one failure signature a reader would otherwise scroll to find,
// paired with the first thing worth trying about it.
type Pattern struct {
	match *regexp.Regexp
	Label string
	Hint  string
}

// Detection is one pattern found in one step. A single signature is reported
// once per step, holding the first line that matched, so a stack trace
// repeating the same message does not bury the other signatures.
type Detection struct {
	Label     string
	Hint      string
	Workflow  string
	Line      string
	StepIndex int
}

// failurePatterns is the detection set, ordered by how specific each signature
// is: the first pattern to match a line claims it.
//
//nolint:gochecknoglobals // a compiled-once lookup table, never assigned to
var failurePatterns = []Pattern{
	{
		Label: "Out of memory",
		Hint:  "The job exceeded the runner's memory. Split the work or move to a larger runner.",
		match: regexp.MustCompile(`(?i)(out of memory|oom-?kill|\bsigkill\b|exit code 137)`),
	},
	{
		Label: "Out of disk",
		Hint:  "The runner filled up. Prune caches or clean the workspace before the failing step.",
		match: regexp.MustCompile(`(?i)(no space left on device|disk quota exceeded)`),
	},
	{
		Label: "Timeout",
		Hint:  "Raise timeout-minutes, or find the step that stopped making progress.",
		match: regexp.MustCompile(`(?i)(timed out|timeout exceeded|context deadline exceeded|` +
			`the job running on runner .* has exceeded)`),
	},
	{
		Label: "Missing secret",
		Hint:  "Check the secret is set on this repository and exposed to the environment the job runs in.",
		match: regexp.MustCompile(`(?i)(secret [^ ]+ (is )?not (found|set)|missing (required )?secret|` +
			`secrets\.[A-Z_]+ is empty)`),
	},
	{
		Label: "Permission denied",
		Hint:  "Check the workflow's permissions block and the token's scopes.",
		match: regexp.MustCompile(`(?i)(permission denied|access denied|resource not accessible by integration|` +
			`bad credentials|\b(401|403) (unauthorized|forbidden)\b|http 403)`),
	},
	{
		Label: "Network failure",
		Hint:  "Usually transient. Re-run the step, and add a retry if it recurs.",
		match: regexp.MustCompile(`(?i)(connection (refused|reset)|temporary failure in name resolution|` +
			`i/o timeout|could not resolve host)`),
	},
}

// Patterns returns the failure signatures Detect looks for.
func Patterns() []Pattern {
	return failurePatterns
}

// Detect reports the failure signatures present in a run's logs.
func Detect(runLogs *RunLogs) []Detection {
	if runLogs == nil {
		return nil
	}

	var detections []Detection

	for _, step := range runLogs.AllSteps() {
		if step == nil {
			continue
		}

		seen := make(map[string]bool, len(failurePatterns))

		var echo CommandEcho

		for _, entry := range step.Entries {
			if echo.Echoed(entry.Content) {
				continue
			}

			for _, pattern := range failurePatterns {
				if seen[pattern.Label] || !pattern.match.MatchString(entry.Content) {
					continue
				}

				seen[pattern.Label] = true

				detections = append(detections, Detection{
					Label:     pattern.Label,
					Hint:      pattern.Hint,
					Workflow:  step.Workflow,
					StepIndex: step.StepIndex,
					Line:      entry.Content,
				})

				break
			}
		}
	}

	return detections
}
