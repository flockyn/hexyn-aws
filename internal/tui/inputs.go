package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
)

// setupLoginInputs builds the three credential entry fields, masking the secret
// key and session token.
func (m *Model) setupLoginInputs() {
	m.inputs = []textinput.Model{
		m.createInput("AWS Access Key ID", ""),
		m.createInput("AWS Secret Access Key", ""),
		m.createInput("AWS Session Token", ""),
	}
	m.inputs[1].EchoMode = textinput.EchoPassword
	m.inputs[1].EchoCharacter = '•'
	m.inputs[2].EchoMode = textinput.EchoPassword
	m.inputs[2].EchoCharacter = '•'
	m.focusIndex = 0
	m.inputs[0].Focus()
}

// cleanRepoName derives a likely SSM repo name from an ECS service name by
// stripping the configured deployment prefixes (see config.RepoNamePrefixes).
func (m Model) cleanRepoName(service string) string {
	repo := service
	for _, prefix := range m.cfg.RepoNamePrefixes() {
		repo = strings.TrimPrefix(repo, prefix)
	}
	return repo
}

// setupInputs builds the action-specific input fields, pre-filling a cleaned
// repo name derived from the selected service.
func (m *Model) setupInputs() {
	m.inputs = nil
	m.focusIndex = 0

	cleanRepo := m.cleanRepoName(m.service)

	// Suffix Path is always the first input for all actions.
	m.inputs = append(m.inputs, m.createInput("Suffix Path (e.g. prod, preprod)", ""))

	switch m.action {
	case "get":
		if m.method == "path" {
			m.inputs = append(m.inputs, m.createInput("SSM Repo Name", cleanRepo))
		}
		m.inputs = append(m.inputs, m.createInput("Output Subdirectory Name", cleanRepo))
	case "put":
		m.inputs = append(m.inputs, m.createInput("SSM Repo Name", cleanRepo))
		m.inputs = append(m.inputs, m.createInput(".env File Name (in input/ dir)", cleanRepo+".env"))
	}
	if len(m.inputs) > 0 {
		m.inputs[0].Focus()
	}
}

// setupConfigInput builds the settings screen's single field, pre-filled with the
// current repo-name prefixes (space-separated).
func (m *Model) setupConfigInput() {
	m.inputs = []textinput.Model{
		m.createInput("Repo Prefixes (space-separated, most specific first)", strings.Join(m.cfg.RepoNamePrefixes(), " ")),
	}
	m.focusIndex = 0
	m.inputs[0].Focus()
}

// createInput builds a text input field. The receiver is unused; it keeps the
// helper grouped on Model alongside the other input setup methods.
func (*Model) createInput(placeholder, value string) textinput.Model {
	t := textinput.New()
	t.Placeholder = placeholder
	t.SetValue(value)
	t.Width = 60
	return t
}

func (m *Model) nextInput() {
	m.inputs[m.focusIndex].Blur()
	m.focusIndex = (m.focusIndex + 1) % len(m.inputs)
	m.inputs[m.focusIndex].Focus()
}

func (m *Model) prevInput() {
	m.inputs[m.focusIndex].Blur()
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = len(m.inputs) - 1
	}
	m.inputs[m.focusIndex].Focus()
}
