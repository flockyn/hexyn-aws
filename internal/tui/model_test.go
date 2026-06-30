package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"hexyn-aws/internal/awsx"
)

func TestNewModelStartsCheckingSession(t *testing.T) {
	assert.Equal(t, stateCheckingSession, newTestModel(t).state)
}

func TestInitReturnsCommand(t *testing.T) {
	assert.NotNil(t, newTestModel(t).Init(), "Init should return a command (spinner tick + session check)")
}

func TestUpdateWindowSizeStoresDimensions(t *testing.T) {
	m := newTestModel(t)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := updated.(Model)

	assert.Equal(t, 78, got.width)  // 80 - 2 (horizontal padding)
	assert.Equal(t, 24, got.height) // 24 - 0 (vertical padding)
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
			assert.Equal(t, tc.want, updated.(Model).state)
		})
	}
}

func TestResultMsgShowsResult(t *testing.T) {
	m := newTestModel(t)
	m.state = stateExecuting

	updated, _ := m.Update(resultMsg{message: "all done"})
	got := updated.(Model)

	assert.Equal(t, stateResult, got.state)
	assert.Equal(t, "all done", got.result)
}
