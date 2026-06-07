package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hexyn-aws/internal/aws"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func updateM(m model, msg tea.Msg) model {
	res, _ := m.Update(msg)
	return res.(model)
}

func TestInitialModel(t *testing.T) {
	m := InitialModel()
	if m.state != stateCheckingSession {
		t.Error("wrong state")
	}
}

func TestTUIFlow(t *testing.T) {
	m := InitialModel()
	m.state = stateMenu

	m.mainMenu.Select(0)
	m = updateM(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateSelectEnv {
		t.Fatal("expected stateSelectEnv")
	}

	m.selector.Select(0)
	m = updateM(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateLoading {
		t.Fatal("expected stateLoading")
	}

	m.state = stateSelectCluster
	m.selector.SetItems([]list.Item{item{title: "c1"}})
	m = updateM(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateLoading {
		t.Fatal("expected stateLoading")
	}

	m.state = stateSelectRepo
	m.repoName = "s1"
	m.action = "get"
	m = updateM(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateSelectMethod {
		t.Fatal("expected stateSelectMethod")
	}

	m.selector.Select(0)
	m = updateM(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateInputs {
		t.Fatal("expected stateInputs")
	}
}

func TestTUIKeys(t *testing.T) {
	m := InitialModel()
	m.state = stateMenu

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("expected quit")
	}

	m2 := updateM(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m2.state != stateLogin {
		t.Error("expected login")
	}

	m3 := updateM(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if m3.state != stateLoading {
		t.Error("expected loading regions")
	}
}

func TestViewAll(t *testing.T) {
	states := []state{stateCheckingSession, stateLogin, stateSelectRegion, stateMenu, stateSelectEnv, stateLoading, stateInputs, stateExecuting, stateResult}
	for _, s := range states {
		m := InitialModel()
		m.state = s
		m.accountAlias = "A"
		m.accountID = "1"
		m.userArn = "ARN"
		m.region = "R"
		m.credSource = "S"
		if s == stateLogin || s == stateInputs {
			m.inputs = []textinput.Model{createInput("P", "V")}
		}
		if s == stateResult {
			m.err = fmt.Errorf("error")
			m.View()
			m.err = nil
			m.result = "success"
		}
		if m.View() == "" {
			t.Errorf("empty view for %v", s)
		}
	}
}

func TestFooter(t *testing.T) {
	m := InitialModel()
	states := []state{stateMenu, stateSelectEnv, stateInputs, stateResult}
	for _, s := range states {
		m.state = s
		if s == stateLogin {
			m.inputs = make([]textinput.Model, 2)
		}
		if m.footerView() == "" {
			t.Errorf("empty footer for %v", s)
		}
	}
}

func TestInputNavigation(t *testing.T) {
	m := InitialModel()
	m.state = stateLogin
	m.setupLoginInputs()

	m = updateM(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIndex != 1 {
		t.Errorf("tab failed, got %d", m.focusIndex)
	}

	m = updateM(m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focusIndex != 0 {
		t.Errorf("shift+tab failed, got %d", m.focusIndex)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Error("expected quit command on esc")
	}
}

func TestListNavigation(t *testing.T) {
	m := InitialModel()
	m.state = stateSelectEnv
	m.selector.SetItems([]list.Item{item{title: "p"}, item{title: "d"}})

	m = updateM(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.state != stateMenu {
		t.Error("esc failed in list")
	}
}

func TestResultTransition(t *testing.T) {
	m := InitialModel()
	m.state = stateResult
	m = updateM(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateMenu {
		t.Error("result enter failed")
	}
}

func TestUpdateMessages(t *testing.T) {
	m := InitialModel()

	m = updateM(m, sessionMsg{arn: "A", source: "S", region: "R"})
	if m.state != stateMenu {
		t.Error("sessionMsg failed")
	}

	m = updateM(m, listMsg{items: []list.Item{item{title: "c1"}}, next: stateSelectCluster})
	if m.state != stateSelectCluster {
		t.Error("listMsg failed")
	}

	m = updateM(m, resultMsg{message: "Success!"})
	if m.state != stateResult {
		t.Error("resultMsg success failed")
	}
}

func TestSetupInputs(t *testing.T) {
	actions := []string{"get", "put"}
	for _, a := range actions {
		m := InitialModel()
		m.action = a
		m.repoName = "test"
		m.setupInputs()
		if len(m.inputs) == 0 {
			t.Errorf("no inputs for action %s", a)
		}
	}
}

func TestUpdateRegionSelector(t *testing.T) {
	m := InitialModel()
	m.state = stateSelectRegion
	m.selector.SetItems([]list.Item{item{title: "us-east-1"}})
	m = updateM(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateMenu || m.region != "us-east-1" {
		t.Error("region selector failed")
	}
}

func TestUpdateInputsEnter(t *testing.T) {
	m := InitialModel()
	m.state = stateInputs
	m.inputs = []textinput.Model{createInput("P", "V")}
	m.focusIndex = 0
	m = updateM(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateExecuting {
		t.Error("inputs enter failed")
	}
}

func TestExecuteActionPut(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "hexyn-tui-test")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	aws.BaseDir = tmpDir

	// Create credentials file to avoid missing error
	_ = os.WriteFile(filepath.Join(tmpDir, "credentials"), []byte("[default]\naws_access_key_id=A\naws_secret_access_key=S"), 0600)

	m := InitialModel()
	m.action = "put"
	m.inputs = []textinput.Model{createInput("SSM", "my-app"), createInput("File", "test.env")}

	_ = os.WriteFile(filepath.Join(tmpDir, "test.env"), []byte("K=V"), 0644)

	cmd := m.executeAction()
	_ = cmd()
}

func TestExecuteActionGetPath(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "hexyn-tui-test")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	aws.BaseDir = tmpDir

	_ = os.WriteFile(filepath.Join(tmpDir, "credentials"), []byte("[default]\naws_access_key_id=A\naws_secret_access_key=S"), 0600)

	m := InitialModel()
	m.action = "get"
	m.method = "path"
	m.environment = "prod"
	m.repoName = "app"
	m.inputs = []textinput.Model{createInput("SSM", "app"), createInput("Out", "app.env")}

	cmd := m.executeAction()
	_ = cmd()
}

func TestItemFilterValue(t *testing.T) {
	i := item{title: "T", desc: "D"}
	if i.FilterValue() != "T D" {
		t.Error("wrong filter value")
	}
}

func TestUpdateListError(t *testing.T) {
	m := InitialModel()
	m = updateM(m, listMsg{err: fmt.Errorf("list error")})
	if m.state != stateResult || m.err == nil {
		t.Error("list error handling failed")
	}
}

func TestUpdateSelectionKeys(t *testing.T) {
	m := InitialModel()
	m.state = stateSelectEnv
	m.selector.SetFilterState(list.Filtering)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("test")})
	if cmd == nil {
		t.Error("list filtering should handle keys")
	}
}

func TestFooterAllStates(t *testing.T) {
	states := []state{stateMenu, stateSelectEnv, stateInputs, stateResult, stateLogin}
	for _, s := range states {
		m := InitialModel()
		m.state = s
		if s == stateLogin {
			m.inputs = make([]textinput.Model, 2)
		}
		if m.footerView() == "" {
			t.Errorf("empty footer for state %v", s)
		}
	}
}

func TestUpdateInputNavWrap(t *testing.T) {
	m := InitialModel()
	m.state = stateLogin
	m.setupLoginInputs()

	m.focusIndex = 0
	m.prevInput()
	if m.focusIndex != 2 {
		t.Error("prevInput wrap failed")
	}

	m.focusIndex = 2
	m.nextInput()
	if m.focusIndex != 0 {
		t.Error("nextInput wrap failed")
	}
}

func TestViewContent(t *testing.T) {
	m := InitialModel()
	m.state = stateMenu
	m.accountAlias = "MyAccount"
	view := m.View()
	if !strings.Contains(view, "Account:      MyAccount") {
		t.Error("Account name not in view")
	}
}

func TestUpdateEscAllStates(t *testing.T) {
	states := []state{stateSelectEnv, stateSelectCluster, stateSelectRepo, stateSelectMethod}
	for _, s := range states {
		m := InitialModel()
		m.state = s
		m = updateM(m, tea.KeyMsg{Type: tea.KeyEsc})
		if m.state != stateMenu {
			t.Errorf("Esc failed for state %v", s)
		}
	}
}

func TestUpdateWindowSize(t *testing.T) {
	m := InitialModel()
	m = updateM(m, tea.WindowSizeMsg{Width: 100, Height: 50})
	if m.width != 100 || m.height != 50 {
		t.Error("WindowSizeMsg failed")
	}
}

func TestInitCmd(t *testing.T) {
	m := InitialModel()
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init returned nil")
	}
}

func TestCheckAWSSessionCmd(t *testing.T) {
	m := InitialModel()
	cmd := m.checkAWSSession()
	_ = cmd() // Should fail but cover
}

func TestFetchRegionsCmd(t *testing.T) {
	m := InitialModel()
	cmd := m.fetchRegions()
	_ = cmd()
}

func TestExecuteActionGetTDF(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "hexyn-tui-test")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	aws.BaseDir = tmpDir
	_ = os.WriteFile(filepath.Join(tmpDir, "credentials"), []byte("[default]\naws_access_key_id=A\naws_secret_access_key=S"), 0600)

	m := InitialModel()
	m.action = "get"
	m.method = "tdf"
	m.environment = "prod"
	m.repoName = "app"
	m.inputs = []textinput.Model{createInput("Out", "app")}

	cmd := m.executeAction()
	_ = cmd()
}

func TestTUILoginSave(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "hexyn-login-save")
	defer func() { _ = os.RemoveAll(tmpDir) }()
	aws.BaseDir = tmpDir

	m := InitialModel()
	m.state = stateLogin
	m.setupLoginInputs()
	m.inputs[0].SetValue("AKIA")
	m.inputs[1].SetValue("SECRET")
	m.inputs[2].SetValue("TOKEN")
	m.focusIndex = 2 // Last field

	m = updateM(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateCheckingSession {
		t.Error("login save failed to transition")
	}
}
