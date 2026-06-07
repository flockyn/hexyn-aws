package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hexyn-aws/internal/aws"
	"hexyn-aws/internal/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1).
			MarginBottom(1).
			Bold(true)

	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	sourceStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	keyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
)

type state int

const (
	stateCheckingSession state = iota
	stateLogin
	stateSelectRegion
	stateMenu
	stateSelectEnv
	stateLoading
	stateSelectCluster
	stateSelectRepo
	stateSelectMethod
	stateInputs
	stateExecuting
	stateResult
)

type item struct {
	title, desc string
	action      string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title + " " + i.desc }

type resultMsg struct {
	message string
	err     error
}

type listMsg struct {
	items []list.Item
	err   error
	next  state
}

type sessionMsg struct {
	arn          string
	accountID    string
	accountAlias string
	profile      string
	source       string
	region       string
	err          error
}

type model struct {
	state        state
	mainMenu     list.Model
	selector     list.Model
	inputs       []textinput.Model
	focusIndex   int
	action       string
	method       string
	environment  string
	cluster      string
	repoName     string
	result       string
	err          error
	spinner      spinner.Model
	width        int
	height       int
	userArn      string
	accountID    string
	accountAlias string
	profileName  string
	credSource   string
	region       string
}

func InitialModel() model {
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

	return model{
		state:    stateCheckingSession,
		mainMenu: mm,
		selector: sel,
		spinner:  s,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.checkAWSSession())
}

func (m model) checkAWSSession() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		arn, id, alias, profile, source, region, err := aws.CheckSession(ctx)
		return sessionMsg{arn: arn, accountID: id, accountAlias: alias, profile: profile, source: source, region: region, err: err}
	}
}

func (m model) fetchRegions() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		regions, err := aws.ListEnabledRegions(ctx)
		if err != nil {
			return listMsg{err: err}
		}
		var items []list.Item
		for _, r := range regions {
			items = append(items, item{title: r, desc: "AWS Region"})
		}
		return listMsg{items: items, next: stateSelectRegion}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.state != stateInputs && m.state != stateLogin {
			if (m.state == stateSelectRegion || m.state == stateSelectEnv || m.state == stateSelectCluster || m.state == stateSelectRepo || m.state == stateSelectMethod) && m.selector.FilterState() == list.Filtering {
				var cmd tea.Cmd
				m.selector, cmd = m.selector.Update(msg)
				return m, cmd
			}

			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "l":
				m.userArn = ""
				m.accountID = ""
				m.accountAlias = ""
				m.credSource = ""
				m.setupLoginInputs()
				m.state = stateLogin
				return m, nil
			case "g":
				if m.state == stateMenu {
					m.state = stateLoading
					return m, m.fetchRegions()
				}
			case "h":
				m.state = stateResult
				m.err = nil
				inputPath := filepath.Join(aws.BaseDir, "input")
				outputPath := filepath.Join(aws.BaseDir, "output")
				m.result = "Hexyn AWS Help\n\n" +
					"L: Change AWS Credentials\n" +
					"G: Change AWS Region\n" +
					"/: Search in lists\n" +
					"ESC: Go back to menu\n" +
					"Q: Exit application\n\n" +
					"Storage Locations:\n" +
					"• PUT: Place .env files in " + inputPath + "/\n" +
					"• GET: Files are saved in " + outputPath + "/"
				return m, nil
			case "esc":
				if m.state == stateLogin || m.state == stateMenu || m.state == stateCheckingSession {
					return m, tea.Quit
				}
				if m.state != stateMenu {
					m.state = stateMenu
					m.err = nil
					m.environment = ""
					m.cluster = ""
					m.repoName = ""
					return m, nil
				}
			case "r":
				if m.state == stateResult {
					m.state = stateMenu
					return m, nil
				}
			}
		} else {
			if msg.String() == "esc" {
				return m, tea.Quit
			}
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
		m.credSource = msg.source
		if msg.err != nil {
			m.err = msg.err
			m.setupLoginInputs()
			m.state = stateLogin
			return m, nil
		}

		// AUTH SUCCESS - Ensure directories are created now
		_ = aws.EnsureDirectories()

		m.userArn = msg.arn
		m.accountID = msg.accountID
		m.accountAlias = msg.accountAlias
		m.profileName = msg.profile
		m.region = msg.region

		if m.region == "" {
			m.state = stateLoading
			return m, m.fetchRegions()
		}

		m.state = stateMenu
		return m, nil

	case listMsg:
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

	case resultMsg:
		m.state = stateResult
		m.result = msg.message
		m.err = msg.err
		return m, nil
	}

	switch m.state {
	case stateLogin:
		return m.updateLogin(msg)
	case stateSelectRegion:
		return m.updateRegionSelector(msg)
	case stateMenu:
		return m.updateMenu(msg)
	case stateSelectEnv:
		return m.updateEnvSelector(msg)
	case stateSelectCluster, stateSelectRepo, stateSelectMethod:
		return m.updateSelector(msg)
	case stateInputs:
		return m.updateInputs(msg)
	case stateResult:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
			m.state = stateMenu
			return m, nil
		}
	case stateLoading, stateCheckingSession:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateRegionSelector(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if m.selector.SelectedItem() == nil {
				return m, nil
			}
			m.region = m.selector.SelectedItem().(item).title
			_ = aws.UpdateRegionInConfig(m.region)
			m.state = stateMenu
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.selector, cmd = m.selector.Update(msg)
	return m, cmd
}

func (m *model) setupLoginInputs() {
	m.inputs = []textinput.Model{
		createInput("AWS Access Key ID", ""),
		createInput("AWS Secret Access Key", ""),
		createInput("AWS Session Token", ""),
	}
	m.inputs[1].EchoMode = textinput.EchoPassword
	m.inputs[1].EchoCharacter = '•'
	m.inputs[2].EchoMode = textinput.EchoPassword
	m.inputs[2].EchoCharacter = '•'
	m.focusIndex = 0
	m.inputs[0].Focus()
}

func (m model) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.focusIndex == len(m.inputs)-1 {
				m.state = stateCheckingSession
				return m, m.saveCredentials()
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

	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m model) saveCredentials() tea.Cmd {
	return func() tea.Msg {
		ak := m.inputs[0].Value()
		sk := m.inputs[1].Value()
		st := m.inputs[2].Value()
		_ = aws.SaveFullCredentials(ak, sk, st, "pending")
		return m.checkAWSSession()()
	}
}

func (m model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			i := m.mainMenu.SelectedItem().(item)
			m.action = i.action
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
	}

	var cmd tea.Cmd
	m.mainMenu, cmd = m.mainMenu.Update(msg)
	return m, cmd
}

func (m model) updateEnvSelector(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if m.selector.SelectedItem() == nil {
				return m, nil
			}
			m.environment = m.selector.SelectedItem().(item).title
			m.state = stateLoading
			m.selector.ResetFilter()
			return m, tea.Batch(m.spinner.Tick, m.fetchClusters())
		}
	}
	var cmd tea.Cmd
	m.selector, cmd = m.selector.Update(msg)
	return m, cmd
}

func (m model) updateSelector(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if m.selector.SelectedItem() == nil {
				return m, nil
			}
			selected := m.selector.SelectedItem().(item).title
			m.selector.ResetFilter()
			switch m.state {
			case stateSelectCluster:
				m.cluster = selected
				m.state = stateLoading
				return m, tea.Batch(m.spinner.Tick, m.fetchRepos())
			case stateSelectRepo:
				m.repoName = selected
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
		}
	}

	var cmd tea.Cmd
	m.selector, cmd = m.selector.Update(msg)
	return m, cmd
}

func (m model) selectionTitle() string {
	switch m.state {
	case stateSelectRegion:
		return "Select AWS Region"
	case stateSelectEnv:
		return "Select Environment"
	case stateSelectCluster:
		return "Select Cluster"
	case stateSelectRepo:
		return "Select Service"
	case stateSelectMethod:
		return "Select Retrieval Method"
	}
	return ""
}

func (m model) fetchClusters() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := aws.NewECSClient(ctx, m.region)
		if err != nil {
			return listMsg{err: err}
		}
		clusters, err := client.ListClusters(ctx)
		if err != nil {
			return listMsg{err: err}
		}
		var items []list.Item
		for _, c := range clusters {
			items = append(items, item{title: c, desc: "ECS Cluster"})
		}
		if len(items) == 0 {
			return listMsg{err: fmt.Errorf("no ECS clusters found")}
		}
		return listMsg{items: items, next: stateSelectCluster}
	}
}

func (m model) fetchRepos() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := aws.NewECSClient(ctx, m.region)
		if err != nil {
			return listMsg{err: err}
		}
		services, err := client.ListServices(ctx, m.cluster)
		if err != nil {
			return listMsg{err: err}
		}
		var items []list.Item
		for _, s := range services {
			items = append(items, item{title: s, desc: "ECS Service"})
		}
		if len(items) == 0 {
			return listMsg{err: fmt.Errorf("no services found in cluster %s", m.cluster)}
		}
		return listMsg{items: items, next: stateSelectRepo}
	}
}

func (m *model) setupInputs() {
	m.inputs = nil
	m.focusIndex = 0

	cleanRepo := m.repoName
	cleanRepo = strings.TrimPrefix(cleanRepo, "service-")
	cleanRepo = strings.TrimPrefix(cleanRepo, "nft-service-")
	cleanRepo = strings.TrimPrefix(cleanRepo, "nft-")

	switch m.action {
	case "get":
		if m.method == "path" {
			m.inputs = append(m.inputs, createInput("SSM Repo Name", cleanRepo))
		}
		m.inputs = append(m.inputs, createInput("Output Subdirectory Name", cleanRepo))
	case "put":
		m.inputs = append(m.inputs, createInput("SSM Repo Name", cleanRepo))
		m.inputs = append(m.inputs, createInput(".env File Path", cleanRepo+".env"))
	}
	if len(m.inputs) > 0 {
		m.inputs[0].Focus()
	}
}

func createInput(placeholder, value string) textinput.Model {
	t := textinput.New()
	t.Placeholder = placeholder
	t.SetValue(value)
	t.Width = 60
	return t
}

func (m model) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.inputs) == 0 && m.state == stateInputs {
		m.state = stateExecuting
		return m, tea.Batch(m.spinner.Tick, m.executeAction())
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.focusIndex == len(m.inputs)-1 {
				m.state = stateExecuting
				return m, tea.Batch(m.spinner.Tick, m.executeAction())
			}
			m.nextInput()
		case "up", "shift+tab":
			m.prevInput()
		case "down", "tab":
			m.nextInput()
		}
	}

	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m *model) nextInput() {
	m.inputs[m.focusIndex].Blur()
	m.focusIndex = (m.focusIndex + 1) % len(m.inputs)
	m.inputs[m.focusIndex].Focus()
}

func (m *model) prevInput() {
	m.inputs[m.focusIndex].Blur()
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = len(m.inputs) - 1
	}
	m.inputs[m.focusIndex].Focus()
}

func (m model) executeAction() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		switch m.action {
		case "get":
			ssmClient, _ := aws.NewSSMClient(ctx, m.region)
			ecsClient, _ := aws.NewECSClient(ctx, m.region)

			outputSubDir := m.inputs[len(m.inputs)-1].Value()
			finalDir := filepath.Join(aws.BaseDir, "output", outputSubDir)
			_ = os.MkdirAll(finalDir, 0755)

			if m.method == "tdf" {
				td, _, tdErr := ecsClient.GetTaskDefinition(ctx, m.cluster, m.repoName)
				if tdErr != nil {
					return resultMsg{err: tdErr}
				}

				if td != nil {
					var secrets []interface{}
					for _, c := range td.ContainerDefinitions {
						for _, s := range c.Secrets {
							secrets = append(secrets, s)
						}
					}
					jsonBytes, _ := json.MarshalIndent(secrets, "", "  ")
					_ = os.WriteFile(filepath.Join(finalDir, "tdf-secrets.json"), jsonBytes, 0644)

					secretMap := ecsClient.GetTaskSecrets(td)
					params, err := ssmClient.GetParametersByNames(ctx, secretMap)
					if err != nil {
						return resultMsg{err: err}
					}

					filesMap := make(map[string]map[string]utils.Parameter)
					for _, p := range params {
						var ssmFullKey string
						for envVar, sName := range secretMap {
							if envVar == p.Name {
								ssmFullKey = sName
								break
							}
						}

						fileName := outputSubDir
						if ssmFullKey != "" {
							parts := strings.Split(ssmFullKey, "/")
							if len(parts) > 1 {
								fileName = parts[len(parts)-2]
							}
						}

						if _, ok := filesMap[fileName]; !ok {
							filesMap[fileName] = make(map[string]utils.Parameter)
						}
						filesMap[fileName][p.Name] = p
					}

					for name, pMap := range filesMap {
						var pList []utils.Parameter
						for _, p := range pMap {
							pList = append(pList, p)
						}

						sort.Slice(pList, func(i, j int) bool {
							return pList[i].Name < pList[j].Name
						})

						var content strings.Builder
						for _, p := range pList {
							content.WriteString(p.Name)
							content.WriteString("=")
							content.WriteString(p.Value)
							if p.Type == "SecureString" {
								content.WriteString(" //secureString")
							}
							content.WriteString("\n")
						}
						_ = os.WriteFile(filepath.Join(finalDir, name+".env"), []byte(content.String()), 0644)
					}
					return resultMsg{message: fmt.Sprintf("Exported tdf-secrets.json and sorted .env files to %s/", finalDir)}
				}
			} else {
				ssmRepo := m.inputs[0].Value()
				params, err := ssmClient.GetParametersByPath(ctx, m.environment, ssmRepo)
				if err != nil {
					return resultMsg{err: err}
				}

				uniqueMap := make(map[string]utils.Parameter)
				for _, p := range params {
					uniqueMap[p.Name] = p
				}

				var pList []utils.Parameter
				for _, p := range uniqueMap {
					pList = append(pList, p)
				}

				sort.Slice(pList, func(i, j int) bool {
					return pList[i].Name < pList[j].Name
				})

				var content strings.Builder
				for _, p := range pList {
					content.WriteString(p.Name)
					content.WriteString("=")
					content.WriteString(p.Value)
					if p.Type == "SecureString" {
						content.WriteString(" //secureString")
					}
					content.WriteString("\n")
				}
				_ = os.WriteFile(filepath.Join(finalDir, "ps.env"), []byte(content.String()), 0644)
				return resultMsg{message: fmt.Sprintf("Exported unique, sorted parameters to %s/ps.env", finalDir)}
			}

		case "put":
			ssmRepo := m.inputs[0].Value()
			fileName := m.inputs[1].Value()

			filePath := fileName
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				filePath = filepath.Join(aws.BaseDir, "input", fileName)
			}

			params, err := utils.ParseEnvFile(filePath)
			if err != nil {
				return resultMsg{err: err}
			}
			ssmClient, _ := aws.NewSSMClient(ctx, m.region)
			success, errs := ssmClient.PutParameters(ctx, m.environment, ssmRepo, params, 10)
			if len(errs) > 0 {
				return resultMsg{message: fmt.Sprintf("Uploaded %d/%d (with errors)", success, len(params)), err: errs[0]}
			}
			return resultMsg{message: fmt.Sprintf("Successfully uploaded %d parameters", success)}
		}
		return resultMsg{message: "Unknown action"}
	}
}

func (m model) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("HEXYN AWS CLI"))
	s.WriteString("\n")

	if m.accountAlias != "" {
		s.WriteString(focusedStyle.Render("Account:      "))
		s.WriteString(m.accountAlias)
		s.WriteString("\n")
	} else if m.accountID != "" {
		s.WriteString(focusedStyle.Render("Account ID:   "))
		s.WriteString(m.accountID)
		s.WriteString("\n")
	}

	if m.userArn != "" {
		s.WriteString(focusedStyle.Render("Identity:     "))
		s.WriteString(m.userArn)
		s.WriteString("\n")
	}
	if m.region != "" {
		s.WriteString(focusedStyle.Render("Region:       "))
		s.WriteString("[")
		s.WriteString(m.region)
		s.WriteString("]\n")
	}

	absPath, _ := filepath.Abs(aws.GetCredentialsPath())
	s.WriteString(sourceStyle.Render("Config Path:  "))
	s.WriteString(absPath)
	s.WriteString("\n\n")

	switch m.state {
	case stateCheckingSession:
		s.WriteString("\n  ")
		s.WriteString(m.spinner.View())
		s.WriteString(" Verifying AWS Session...")

	case stateLogin:
		s.WriteString(focusedStyle.Render("🔑 AWS Login Required"))
		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("Session missing or expired. Please provide your keys:"))
		s.WriteString("\n\n")

		if m.err != nil && m.err.Error() != "MISSING" && m.err.Error() != "EXPIRED" {
			s.WriteString(errorStyle.Render("Error: " + m.err.Error()))
			s.WriteString("\n\n")
		}

		for _, input := range m.inputs {
			s.WriteString(helpStyle.Render(input.Placeholder))
			s.WriteString("\n")
			s.WriteString(input.View())
			s.WriteString("\n\n")
		}

	case stateSelectRegion:
		s.WriteString(m.selector.View())

	case stateMenu:
		s.WriteString(m.mainMenu.View())
	case stateSelectEnv, stateSelectCluster, stateSelectRepo, stateSelectMethod:
		s.WriteString(m.selector.View())
	case stateLoading:
		s.WriteString("\n  ")
		s.WriteString(m.spinner.View())
		s.WriteString(" Fetching from AWS...")
	case stateInputs:
		s.WriteString(focusedStyle.Render("Confirm details:"))
		s.WriteString("\n\n")
		for _, input := range m.inputs {
			s.WriteString(helpStyle.Render(input.Placeholder))
			s.WriteString("\n")
			s.WriteString(input.View())
			s.WriteString("\n\n")
		}
	case stateExecuting:
		s.WriteString(m.spinner.View())
		s.WriteString(" Executing ")
		s.WriteString(m.action)
		s.WriteString(" operation...")
	case stateResult:
		if m.err != nil {
			s.WriteString(errorStyle.Render("Error: " + m.err.Error()))
		} else {
			s.WriteString(successStyle.Render("Success!"))
			s.WriteString("\n\n")
			s.WriteString(m.result)
		}
	}

	s.WriteString("\n\n")
	s.WriteString(m.footerView())

	return s.String()
}

func (m model) footerView() string {
	var items []string

	items = append(items, keyStyle.Render("Q")+" Quit")

	switch m.state {
	case stateMenu:
		items = append(items, keyStyle.Render("L")+" Login")
		items = append(items, keyStyle.Render("G")+" Region")
		items = append(items, keyStyle.Render("H")+" Help")
	case stateSelectEnv, stateSelectCluster, stateSelectRepo, stateSelectMethod:
		items = append(items, keyStyle.Render("ESC")+" Back")
		items = append(items, keyStyle.Render("/")+" Search")
		items = append(items, keyStyle.Render("H")+" Help")
	case stateInputs, stateLogin:
		items = append(items, keyStyle.Render("ESC")+" Quit")
		items = append(items, keyStyle.Render("ENTER")+" Next/Save")
		if len(m.inputs) > 1 {
			items = append(items, keyStyle.Render("TAB")+" Move")
		}
	case stateResult:
		items = append(items, keyStyle.Render("ENTER")+" Menu")
	}

	return strings.Join(items, " • ")
}
