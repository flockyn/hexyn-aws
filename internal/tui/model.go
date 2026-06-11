// Package tui is the inbound Bubble Tea adapter. It renders the interactive
// interface and translates user actions into calls on *secrets.Service.
package tui

import (
	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/config"
	"hexyn-aws/internal/secrets"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type item struct {
	title, desc string
	action      string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title + " " + i.desc }

// Model is the Bubble Tea model holding all TUI state.
type Model struct {
	cfg   *config.Provider
	cmds  *commandRunner
	state state

	session awsx.Session

	mainMenu list.Model
	selector list.Model
	spinner  spinner.Model
	inputs   []textinput.Model

	focusIndex int
	action     string
	method     string
	env        string
	cluster    string
	service    string
	result     string
	err        error

	width  int
	height int
}

// NewModel builds the initial TUI model wired to the given service and config.
func NewModel(svc *secrets.Service, cfg *config.Provider) Model {
	mItems := []list.Item{
		item{title: "Get Parameters", desc: "Retrieve SSM parameters to .env", action: "get"},
		item{title: "Put Parameters", desc: "Upload .env to SSM", action: "put"},
	}

	mm := list.New(mItems, list.NewDefaultDelegate(), 0, 0)
	mm.Title = "Hexyn AWS"
	mm.SetShowStatusBar(false)
	mm.SetFilteringEnabled(false)
	mm.SetShowHelp(false)
	mm.Styles.Title = titleStyle

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	sel := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	sel.SetShowStatusBar(true)
	sel.SetFilteringEnabled(true)
	sel.SetShowHelp(false)
	sel.Styles.Title = titleStyle

	return Model{
		cfg:      cfg,
		cmds:     &commandRunner{svc: svc},
		state:    stateCheckingSession,
		mainMenu: mm,
		selector: sel,
		spinner:  s,
	}
}

// Init kicks off the spinner and the initial session check.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.cmds.checkSession())
}

// Update routes global keys and messages, then delegates to the per-state handler.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if handled, model, cmd := m.handleGlobalKey(msg); handled {
			return model, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.mainMenu.SetSize(msg.Width, msg.Height-12)
		m.selector.SetSize(msg.Width, msg.Height-12)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case sessionMsg:
		return m.handleSessionMsg(msg)

	case listMsg:
		return m.handleListMsg(msg)

	case resultMsg:
		return m.handleResultMsg(msg)
	}

	return m.updateForState(msg)
}

// updateForState dispatches a message to the handler for the current state.
func (m Model) updateForState(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateLogin:
		return m.updateLogin(msg)
	case stateSelectRegion:
		return m.updateRegionSelector(msg)
	case stateMenu:
		return m.updateMenu(msg)
	case stateSelectEnv:
		return m.updateEnvSelector(msg)
	case stateSelectCluster, stateSelectService, stateSelectMethod:
		return m.updateSelector(msg)
	case stateInputs:
		return m.updateInputs(msg)
	case stateResult:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
			m.state = stateMenu
			return m, nil
		}
	}
	return m, nil
}
