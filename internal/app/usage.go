package app

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/keyusage"

	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

// UsageApp names the usage log's directory, beside the frecency store's.
const UsageApp = "lazydispatch"

// StaleAfter is how long a key may go unpressed before `export keys` lists it as neglected.
const StaleAfter = 30 * 24 * time.Hour

const (
	ctxChains    = "chains"
	ctxCommand   = "command-bar"
	ctxConfig    = "config"
	ctxFlaky     = "flaky"
	ctxHistory   = "history"
	ctxLive      = "live"
	ctxRuns      = "runs"
	ctxWorkflows = "workflows"
)

// WithUsage records the keys pressed outside text fields into rec.
func WithUsage(rec *keyusage.Recorder) Option {
	return func(o *options) { o.usage = rec }
}

// paneContext names where a key will land: the focused pane, or the right
// panel's active tab.
func (m Model) paneContext() string {
	switch m.focused {
	case PaneWorkflows:
		return ctxWorkflows
	case PaneChains:
		return ctxChains
	case PaneConfig:
		return ctxConfig
	case PaneRight:
		return tabContext(m.rightPanel.ActiveTab())
	}

	return ctxWorkflows
}

func tabContext(tab panes.RightTab) string {
	switch tab {
	case panes.TabLive:
		return ctxLive
	case panes.TabHistory:
		return ctxHistory
	case panes.TabFlaky:
		return ctxFlaky
	case panes.TabRuns:
	}

	return ctxRuns
}

// recordKey logs a key the focused pane or the top modal is about to receive.
func (m Model) recordKey(msg tea.KeyPressMsg) {
	if top := m.modalStack.Current(); top != nil {
		if modal.Records(top, msg.String()) {
			m.usage.Key(modal.KeyContext(top), msg.String())
		}

		return
	}

	m.usage.Key(m.paneContext(), msg.String())
}

// Keymap is every context recordKey names, built from the key tables the panes
// and modals dispatch on.
func Keymap() keyusage.Keymap {
	km := modal.Keymap()
	keys := DefaultKeyMap()

	left := []key.Binding{keys.PaneRight}
	right := []key.Binding{keys.PaneLeft, keys.PaneRight, keys.Reload}
	global := []key.Binding{
		keys.Actions, keys.Branch, keys.Chain, keys.Command, keys.Down, keys.Enter, keys.Escape, keys.Help,
		keys.LiveView, keys.Quit, keys.ShiftTab, keys.Tab, keys.TabNext, keys.TabPrev, keys.Up, keys.Watch,
	}

	km[ctxWorkflows] = join(global, left, []key.Binding{keys.Space}, digits(keys.WorkflowKeys()[1:], "select workflow"))
	km[ctxWorkflows] = append(km[ctxWorkflows], toBinding(keys.Workflow0))
	km[ctxChains] = join(global, left)
	km[ctxConfig] = join(global, left, []key.Binding{keys.Copy, keys.Edit, keys.Filter, keys.Reset},
		digits(append(keys.InputKeys()[:9:9], keys.Input0), "edit input"))
	km[ctxRuns] = join(global, right, []key.Binding{keys.Scope})
	km[ctxLive] = join(global, right, []key.Binding{keys.Clear, keys.ClearAll, keys.Space})
	km[ctxHistory] = join(global, right, []key.Binding{keys.ViewLogs})
	km[ctxFlaky] = join(global, right)

	return km
}

func join(groups ...[]key.Binding) []keyusage.Binding {
	var out []keyusage.Binding

	for _, g := range groups {
		for _, b := range g {
			out = append(out, toBinding(b))
		}
	}

	return out
}

func toBinding(b key.Binding) keyusage.Binding {
	return keyusage.Binding{Keys: b.Keys(), What: b.Help().Desc}
}

// digits is the numbered bindings as one, since each reaches the row its digit names.
func digits(bs []key.Binding, what string) []key.Binding {
	var keys []string

	for _, b := range bs {
		keys = append(keys, b.Keys()...)
	}

	return []key.Binding{key.NewBinding(key.WithKeys(keys...), key.WithHelp("1-9", what))}
}
