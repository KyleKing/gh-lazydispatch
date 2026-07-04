package app

import (
	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

// FetchLogsMsg requests fetching logs for a chain or run.
type FetchLogsMsg struct {
	ChainState *chain.ChainState
	Workflow   string
	Branch     string
	RunID      int64
	ErrorsOnly bool
}

// LogsFetchedMsg contains fetched logs or an error.
type LogsFetchedMsg struct {
	Error      error
	Logs       *logs.RunLogs
	Workflow   string
	RunID      int64
	ErrorsOnly bool
}

// ShowLogsViewerMsg opens the logs viewer modal.
type ShowLogsViewerMsg struct {
	Logs       *logs.RunLogs
	Workflow   string
	RunID      int64
	ErrorsOnly bool
}

// StartLogStreamMsg begins streaming logs for an active run.
type StartLogStreamMsg struct {
	Workflow   string
	RunID      int64
	AutoScroll bool
}

// LogStreamUpdateMsg contains new log content from streaming.
type LogStreamUpdateMsg struct {
	Update logs.StreamUpdate
}

// StopLogStreamMsg stops streaming logs for a run.
type StopLogStreamMsg struct {
	RunID int64
}
