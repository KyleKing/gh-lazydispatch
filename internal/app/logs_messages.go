package app

import (
	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

// FetchLogsMsg requests fetching logs for a chain or run.
type FetchLogsMsg struct {
	ChainState *chain.ChainState
	RunID      int64
	Workflow   string
	Branch     string
	ErrorsOnly bool
}

// LogsFetchedMsg contains fetched logs or an error.
type LogsFetchedMsg struct {
	Logs       *logs.RunLogs
	ErrorsOnly bool
	RunID      int64
	Workflow   string
	Error      error
}

// ShowLogsViewerMsg opens the logs viewer modal.
type ShowLogsViewerMsg struct {
	Logs       *logs.RunLogs
	ErrorsOnly bool
	RunID      int64
	Workflow   string
}

// StartLogStreamMsg begins streaming logs for an active run.
type StartLogStreamMsg struct {
	RunID      int64
	Workflow   string
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
