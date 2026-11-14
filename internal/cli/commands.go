package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/kyleking/aragonite/keyusage"
)

const programName = "gh-lazydispatch"

type command struct {
	run   func(args []string, stdout, stderr io.Writer) error
	flags func(fs *flag.FlagSet)
	name  string
	what  string
}

// RootFlags declares the flags the bare command takes, for main to parse.
func RootFlags(fs *flag.FlagSet) (*bool, *bool) {
	showVersion := fs.Bool("version", false, "Show version")
	fs.BoolVar(showVersion, "v", false, "Show version (shorthand)")
	showHelp := fs.Bool("help", false, "Show help")
	fs.BoolVar(showHelp, "h", false, "Show help (shorthand)")

	return showVersion, showHelp
}

func exportCommands() []command {
	return []command{
		{name: "chains", what: "chains defined in .github/lazydispatch.yml", run: exportChains},
		{
			name: "ci-report", what: "one shareable CI report", run: exportCIReport,
			flags: func(fs *flag.FlagSet) { ciReportFlags(fs) },
		},
		{
			name: "ci-timing", what: "how long CI takes per commit", run: exportCITiming,
			flags: func(fs *flag.FlagSet) { ciTimingFlags(fs) },
		},
		{
			name: "diagnose", what: "why a run failed", run: exportDiagnose,
			flags: func(fs *flag.FlagSet) { diagnoseFlags(fs) },
		},
		{
			name: "job-failures", what: "failure rate of each job", run: exportJobFailures,
			flags: func(fs *flag.FlagSet) { jobFailureFlags(fs) },
		},
		{name: "keys", what: "keys and commands never used", run: exportKeys},
		{
			name: "logs", what: "one run's logs, parsed into steps", run: exportLogs,
			flags: func(fs *flag.FlagSet) { logsFlags(fs) },
		},
		{
			name: "runs", what: "recent workflow runs", run: exportRuns,
			flags: func(fs *flag.FlagSet) { runsFlags(fs) },
		},
		{name: "workflows", what: "dispatchable workflows and their inputs", run: exportWorkflows},
	}
}

// Root is the command tree key usage records against, walked from the flag
// sets the commands parse with.
func Root() keyusage.Command {
	subs := make([]keyusage.Command, 0, len(exportCommands()))
	for _, c := range exportCommands() {
		subs = append(subs, keyusage.Command{Name: c.name, What: c.what, Flags: flagNames(c.flags)})
	}

	return keyusage.Command{
		Name:  programName,
		What:  "launch the TUI",
		Flags: flagNames(func(fs *flag.FlagSet) { RootFlags(fs) }),
		Subs: []keyusage.Command{
			{Name: "export", What: "read Actions without dispatching", Subs: subs},
			{
				Name:  cmdWatch,
				What:  "wait out a run and write its failure digest",
				Flags: flagNames(func(fs *flag.FlagSet) { watchFlags(fs) }),
			},
		},
	}
}

func flagNames(declare func(fs *flag.FlagSet)) []string {
	if declare == nil {
		return nil
	}

	fs := newFlagSet("")
	declare(fs)

	var names []string

	fs.VisitAll(func(f *flag.Flag) {
		if len(f.Name) == 1 {
			names = append(names, "-"+f.Name)
		} else {
			names = append(names, "--"+f.Name)
		}
	})

	return names
}

func findCommand(name string) (command, bool) {
	for _, c := range exportCommands() {
		if c.name == name {
			return c, true
		}
	}

	return command{}, false
}

func runExportCommand(name string, args []string, stdout, stderr io.Writer) error {
	c, ok := findCommand(name)
	if !ok {
		return fmt.Errorf("%w: unknown export command %q", ErrUsage, name)
	}

	return c.run(args, stdout, stderr)
}
