package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hexyn-aws/internal/awsx"
	"hexyn-aws/test/fixtures"
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

func TestPutSubmitParsesBeforeConfirming(t *testing.T) {
	m := newTestModel(t)
	m.action = "put"
	m.service = "service-orders"
	m.setupInputs() // SSM Repo Name + .env File Name
	m.focusIndex = len(m.inputs) - 1

	updated, cmd := m.updateInputs(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	assert.Equal(t, stateLoading, got.state, "put should parse the file before confirming, not execute immediately")
	require.NotNil(t, cmd, "expected a preview command")
}

func TestPreviewMsgRoutesToConfirm(t *testing.T) {
	m := newTestModel(t)
	params := []awsx.Parameter{{Name: "A", Value: "1"}}

	updated, _ := m.Update(previewMsg{params: params})
	got := updated.(Model)

	assert.Equal(t, stateConfirmPut, got.state)
	assert.Equal(t, params, got.previewParams)
}

func TestPreviewMsgErrorRoutesToResult(t *testing.T) {
	m := newTestModel(t)

	updated, _ := m.Update(previewMsg{err: errors.New("bad file")})
	got := updated.(Model)

	assert.Equal(t, stateResult, got.state)
	assert.Error(t, got.err)
}

func TestConfirmPutEnterExecutes(t *testing.T) {
	m := newTestModel(t)
	m.action = "put"
	m.service = "service-orders"
	m.setupInputs()
	m.state = stateConfirmPut
	m.previewParams = []awsx.Parameter{{Name: "A"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	assert.Equal(t, stateExecuting, got.state)
	require.NotNil(t, cmd, "expected an execute command on confirm")
}

func TestEscFromConfirmPutReturnsToMenu(t *testing.T) {
	m := newTestModel(t)
	m.state = stateConfirmPut

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stateMenu, updated.(Model).state)
}

func TestSettingsKeyOpensConfigFromAnyScreen(t *testing.T) {
	for _, st := range []state{stateMenu, stateSelectService, stateResult} {
		m := newTestModel(t)
		m.state = st

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		got := updated.(Model)

		assert.Equalf(t, stateConfig, got.state, "pressing 's' on state %d should open settings", st)
		require.Lenf(t, got.inputs, 1, "the settings screen has a single prefix field (state %d)", st)
	}
}

func TestSettingsKeyIgnoredWhileTyping(t *testing.T) {
	m := newTestModel(t)
	m.action = "put"
	m.service = "orders"
	m.setupInputs() // stateInputs has text fields
	m.state = stateInputs

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := updated.(Model)

	assert.Equal(t, stateInputs, got.state, "'s' should type into the field, not open settings")
	assert.Contains(t, got.inputs[0].Value(), "s", "the keystroke should reach the input")
}

func TestConfigSaveWritesAndShowsResult(t *testing.T) {
	fixtures.Chdir(t, t.TempDir()) // isolate the config-file write
	t.Setenv("HEXYN_REPO_PREFIXES", "")

	m := newTestModel(t)
	m.setupConfigInput()
	m.state = stateConfig
	m.inputs[0].SetValue("team-service- team-")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	assert.Equal(t, stateResult, got.state)
	assert.Equal(t, []string{"team-service-", "team-"}, got.cfg.RepoNamePrefixes(),
		"saved prefixes should be active and persisted")
}

func TestEscFromConfigReturnsToMenu(t *testing.T) {
	m := newTestModel(t)
	m.setupConfigInput()
	m.state = stateConfig

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stateMenu, updated.(Model).state)
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
