package tui

import (
	"strings"
	"testing"

	"hexyn-aws/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	return NewModel(nil, config.New(true)) // svc unused by the transitions under test
}

func TestMenuEnterAdvancesToEnvSelect(t *testing.T) {
	m := newTestModel(t)
	m.state = stateMenu

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if got.state != stateSelectEnv {
		t.Fatalf("expected stateSelectEnv, got %d", got.state)
	}
	if got.action != "get" {
		t.Errorf("expected action 'get' (first menu item), got %q", got.action)
	}
}

func TestEscFromMenuQuits(t *testing.T) {
	m := newTestModel(t)
	m.state = stateMenu

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("expected QuitMsg from esc on menu")
	}
}

func TestEscFromSelectorReturnsToMenu(t *testing.T) {
	m := newTestModel(t)
	m.state = stateSelectCluster

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(Model).state != stateMenu {
		t.Fatal("expected esc from a selector state to return to menu")
	}
}

func TestEscFromConfirmationReturnsToMenu(t *testing.T) {
	m := newTestModel(t)
	m.state = stateInputs
	m.action = "get"
	m.method = "tdf"
	m.setupInputs() // confirmation screen has input fields

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(Model).state != stateMenu {
		t.Fatal("expected esc from the confirmation screen to return to menu")
	}
}

func TestHelpKeyShowsHelpResult(t *testing.T) {
	m := newTestModel(t)
	m.state = stateMenu

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	got := updated.(Model)

	if got.state != stateResult {
		t.Fatalf("expected stateResult, got %d", got.state)
	}
	if !strings.Contains(got.result, "Hexyn AWS Help") {
		t.Errorf("expected help text, got %q", got.result)
	}
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
		if got := m.selectionTitle(); got != want {
			t.Errorf("state %d: got %q want %q", st, got, want)
		}
	}
}
