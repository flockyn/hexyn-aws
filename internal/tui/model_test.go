package tui

import (
	"testing"

	"hexyn-aws/internal/awsx"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelStartsCheckingSession(t *testing.T) {
	if newTestModel(t).state != stateCheckingSession {
		t.Fatal("expected initial state stateCheckingSession")
	}
}

func TestInitReturnsCommand(t *testing.T) {
	if newTestModel(t).Init() == nil {
		t.Fatal("Init should return a command (spinner tick + session check)")
	}
}

func TestUpdateWindowSizeStoresDimensions(t *testing.T) {
	m := newTestModel(t)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := updated.(Model)

	if got.width != 80 || got.height != 24 {
		t.Errorf("expected 80x24, got %dx%d", got.width, got.height)
	}
}

func TestSessionMsgRoutes(t *testing.T) {
	cases := []struct {
		name string
		msg  sessionMsg
		want state
	}{
		{"missing creds -> login", sessionMsg{err: awsx.ErrCredentialsMissing}, stateLogin},
		{"no region -> loading", sessionMsg{session: awsx.Session{AccountID: "1"}}, stateLoading},
		{"with region -> menu", sessionMsg{session: awsx.Session{Region: "ap-southeast-3"}}, stateMenu},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t)
			updated, _ := m.Update(tc.msg)
			if got := updated.(Model).state; got != tc.want {
				t.Errorf("got state %d, want %d", got, tc.want)
			}
		})
	}
}

func TestResultMsgShowsResult(t *testing.T) {
	m := newTestModel(t)
	m.state = stateExecuting

	updated, _ := m.Update(resultMsg{message: "all done"})
	got := updated.(Model)

	if got.state != stateResult {
		t.Fatalf("expected stateResult, got %d", got.state)
	}
	if got.result != "all done" {
		t.Errorf("result not stored: %q", got.result)
	}
}
