package app

import (
	"bytes"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
)

func waitForOutput(t *testing.T, tm *teatest.TestModel, want string) {
	t.Helper()

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(want))
	}, teatest.WithDuration(3*time.Second))
}

func TestAppBootAndQuit(t *testing.T) {
	tm := teatest.NewTestModel(t, newRenderModel(), teatest.WithInitialTermSize(120, 40))

	waitForOutput(t, tm, "Deploy")

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestAppTinyTerminalFallback(t *testing.T) {
	tm := teatest.NewTestModel(t, newRenderModel(), teatest.WithInitialTermSize(40, 10))

	waitForOutput(t, tm, "Terminal too small")

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestAppHelpModalLifecycle(t *testing.T) {
	tm := teatest.NewTestModel(t, newRenderModel(), teatest.WithInitialTermSize(120, 40))

	waitForOutput(t, tm, "Deploy")

	tm.Send(tea.KeyPressMsg{Code: '?', Text: "?"})
	waitForOutput(t, tm, "Keyboard Shortcuts")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
