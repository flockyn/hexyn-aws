package tui

import (
	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/secrets"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// isSelectorState reports whether the current state is driven by the shared selector list.
func (m Model) isSelectorState() bool {
	switch m.state {
	case stateSelectRegion, stateSelectEnv, stateSelectCluster, stateSelectService, stateSelectMethod:
		return true
	default:
		return false
	}
}

// handleGlobalKey processes keys available across most states. It returns
// handled=false when the key should fall through to the per-state handler.
func (m Model) handleGlobalKey(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	if m.state == stateInputs || m.state == stateLogin {
		// While typing, only ESC is treated as a shortcut (it can't be typed into
		// a field); every other key falls through to the input handler.
		if msg.String() == "esc" {
			return m.handleEsc()
		}
		return false, m, nil
	}

	if m.isSelectorState() && m.selector.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.selector, cmd = m.selector.Update(msg)
		return true, m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return true, m, tea.Quit
	case "l":
		m.session = awsx.Session{}
		m.setupLoginInputs()
		m.state = stateLogin
		return true, m, nil
	case "g":
		if m.state == stateMenu {
			m.state = stateLoading
			return true, m, m.cmds.listRegions()
		}
	case "h":
		m.showHelp()
		return true, m, nil
	case "esc":
		return m.handleEsc()
	case "r":
		if m.state == stateResult {
			m.state = stateMenu
			return true, m, nil
		}
	}
	return false, m, nil
}

// handleEsc implements the context-sensitive escape behaviour.
func (m Model) handleEsc() (bool, tea.Model, tea.Cmd) {
	if m.state == stateLogin || m.state == stateMenu || m.state == stateCheckingSession {
		return true, m, tea.Quit
	}
	if m.state != stateMenu {
		m.state = stateMenu
		m.err = nil
		return true, m, nil
	}
	return false, m, nil
}

// showHelp switches to the result view rendered with the help text.
func (m *Model) showHelp() {
	m.state = stateResult
	m.err = nil
	m.result = "Hexyn AWS Help\n\n" +
		"L: Change AWS Credentials\n" +
		"G: Change AWS Region\n" +
		"/: Search in lists\n" +
		"ESC: Go back to menu\n" +
		"Q: Exit application\n\n" +
		"Storage Locations:\n" +
		"• PUT: Place .env files in " + m.cfg.InputDir() + "/\n" +
		"• GET: Files are saved in " + m.cfg.OutputDir() + "/"
}

func (m Model) handleSessionMsg(msg sessionMsg) (tea.Model, tea.Cmd) {
	m.session = msg.session
	if msg.err != nil {
		m.err = msg.err
		m.setupLoginInputs()
		m.state = stateLogin
		return m, nil
	}
	if m.session.Region == "" {
		m.state = stateLoading
		return m, m.cmds.listRegions()
	}
	m.state = stateMenu
	return m, nil
}

func (m Model) handleListMsg(msg listMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.state = stateResult
		m.err = msg.err
		return m, nil
	}
	m.state = msg.next
	m.selector.SetItems(msg.items)
	m.selector.Title = m.selectionTitle()
	m.selector.Select(0)
	m.selector.ResetFilter()
	return m, nil
}

func (m Model) handleResultMsg(msg resultMsg) (tea.Model, tea.Cmd) {
	if m.state == stateSelectRegion && msg.err == nil {
		m.state = stateMenu
		return m, nil
	}
	m.state = stateResult
	m.result = msg.message
	m.err = msg.err
	return m, nil
}

func (m Model) selectionTitle() string {
	switch m.state {
	case stateSelectRegion:
		return "Select AWS Region"
	case stateSelectEnv:
		return "Select Environment"
	case stateSelectCluster:
		return "Select Cluster"
	case stateSelectService:
		return "Select Service"
	case stateSelectMethod:
		return "Select Retrieval Method"
	}
	return ""
}

func (m Model) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			if m.focusIndex == len(m.inputs)-1 {
				m.state = stateCheckingSession
				creds := awsx.Credentials{
					AccessKeyID:     m.inputs[0].Value(),
					SecretAccessKey: m.inputs[1].Value(),
					SessionToken:    m.inputs[2].Value(),
				}
				return m, m.cmds.saveCredentials(creds, "pending")
			}
			m.nextInput()
			return m, nil
		case "up", "shift+tab":
			m.prevInput()
			return m, nil
		case "down", "tab":
			m.nextInput()
			return m, nil
		}
	}
	return m, m.updateInputFields(msg)
}

func (m Model) updateRegionSelector(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		if m.selector.SelectedItem() == nil {
			return m, nil
		}
		m.session.Region = m.selector.SelectedItem().(item).title
		return m, m.cmds.updateRegion(m.session.Region)
	}
	var cmd tea.Cmd
	m.selector, cmd = m.selector.Update(msg)
	return m, cmd
}

func (m Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		m.action = m.mainMenu.SelectedItem().(item).action
		envs := []list.Item{
			item{title: "prod", desc: "Production Environment"},
			item{title: "preprod", desc: "Pre-production Environment"},
		}
		m.selector.SetItems(envs)
		m.selector.Title = "Select Environment"
		m.selector.Select(0)
		m.selector.ResetFilter()
		m.state = stateSelectEnv
		return m, nil
	}
	var cmd tea.Cmd
	m.mainMenu, cmd = m.mainMenu.Update(msg)
	return m, cmd
}

func (m Model) updateEnvSelector(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		if m.selector.SelectedItem() == nil {
			return m, nil
		}
		m.env = m.selector.SelectedItem().(item).title
		m.state = stateLoading
		return m, m.cmds.listClusters(m.session.Region)
	}
	var cmd tea.Cmd
	m.selector, cmd = m.selector.Update(msg)
	return m, cmd
}

func (m Model) updateSelector(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		if m.selector.SelectedItem() == nil {
			return m, nil
		}
		return m.selectCurrent()
	}
	var cmd tea.Cmd
	m.selector, cmd = m.selector.Update(msg)
	return m, cmd
}

// selectCurrent advances the flow based on the item chosen in a selector state.
func (m Model) selectCurrent() (tea.Model, tea.Cmd) {
	selected := m.selector.SelectedItem().(item).title
	m.selector.ResetFilter()
	switch m.state {
	case stateSelectCluster:
		m.cluster = selected
		m.state = stateLoading
		return m, m.cmds.listServices(m.session.Region, m.cluster)
	case stateSelectService:
		m.service = selected
		if m.action == "get" {
			m.state = stateSelectMethod
			m.selector.SetItems([]list.Item{
				item{title: "From Task Definition", desc: "Get exact secrets defined in TDF (recommended)", action: "tdf"},
				item{title: "By Path Prefix", desc: "Get all parameters under /env/repo/ path", action: "path"},
			})
			m.selector.Title = "Select Retrieval Method"
			m.selector.Select(0)
			return m, nil
		}
		m.setupInputs()
		m.state = stateInputs
		return m, nil
	case stateSelectMethod:
		m.method = m.selector.SelectedItem().(item).action
		m.setupInputs()
		m.state = stateInputs
		return m, nil
	}
	return m, nil
}

func (m Model) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.inputs) == 0 && m.state == stateInputs {
		m.state = stateExecuting
		return m, m.executeAction()
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			if m.focusIndex == len(m.inputs)-1 {
				m.state = stateExecuting
				return m, m.executeAction()
			}
			m.nextInput()
		case "up", "shift+tab":
			m.prevInput()
		case "down", "tab":
			m.nextInput()
		}
	}
	return m, m.updateInputFields(msg)
}

// updateInputFields forwards a message to every text input and batches their commands.
func (m Model) updateInputFields(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m Model) executeAction() tea.Cmd {
	switch m.action {
	case "get":
		outDir := m.inputs[len(m.inputs)-1].Value()
		if m.method == "tdf" {
			return m.cmds.getByTaskDef(secrets.TaskTarget{
				Region: m.session.Region, Cluster: m.cluster, Service: m.service, OutputDir: outDir,
			})
		}
		return m.cmds.getByPath(secrets.ParamTarget{
			Env: m.env, Repo: m.inputs[0].Value(), Region: m.session.Region,
		}, outDir)
	case "put":
		target := secrets.ParamTarget{Env: m.env, Repo: m.inputs[0].Value(), Region: m.session.Region}
		return m.cmds.putParameters(target, m.inputs[1].Value())
	}
	return nil
}
