package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMenuEnterAdvancesToEnvSelect(t *testing.T) {
	m := newTestModel(t)
	m.state = stateMenu

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	assert.Equal(t, stateSelectEnv, got.state)
	assert.Equal(t, "get", got.action, "expected action 'get' (first menu item)")
}

func TestEscFromMenuQuits(t *testing.T) {
	m := newTestModel(t)
	m.state = stateMenu

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd, "expected a quit command")
	_, ok := cmd().(tea.QuitMsg)
	assert.True(t, ok, "expected QuitMsg from esc on menu")
}

func TestEscFromSelectorReturnsToMenu(t *testing.T) {
	m := newTestModel(t)
	m.state = stateSelectCluster

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stateMenu, updated.(Model).state, "expected esc from a selector state to return to menu")
}

func TestEscFromConfirmationReturnsToMenu(t *testing.T) {
	m := newTestModel(t)
	m.state = stateInputs
	m.action = "get"
	m.method = "tdf"
	m.setupInputs() // confirmation screen has input fields

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stateMenu, updated.(Model).state, "expected esc from the confirmation screen to return to menu")
}

func TestHelpKeyShowsHelpResult(t *testing.T) {
	m := newTestModel(t)
	m.state = stateMenu

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	got := updated.(Model)

	assert.Equal(t, stateResult, got.state)
	assert.Contains(t, got.result, "Hexyn AWS Help")
}

func TestSelectionTitle(t *testing.T) {
	cases := map[state]string{
		stateSelectRegion:  "Select AWS Region",
		stateSelectEnv:     "Select Environment",
		stateSelectCluster: "Select Cluster",
		stateSelectService: "Select Service",
		stateSelectMethod:  "Select Retrieval Method",
	}
	for st, want := range cases {
		m := newTestModel(t)
		m.state = st
		assert.Equalf(t, want, m.selectionTitle(), "state %d", st)
	}
}
