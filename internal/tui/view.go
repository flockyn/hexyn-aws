package tui

import (
	"errors"
	"path/filepath"
	"strings"

	"hexyn-aws/internal/awsx"
)

// View renders the full screen for the current state.
func (m Model) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("HEXYN AWS CLI"))
	s.WriteString("\n")
	m.writeSessionHeader(&s)
	m.writeBody(&s)

	s.WriteString("\n\n")
	s.WriteString(m.footerView())
	return s.String()
}

// writeSessionHeader renders the account / identity / region / config-path block.
func (m Model) writeSessionHeader(s *strings.Builder) {
	if m.session.DisplayName() != "" {
		s.WriteString(focusedStyle.Render("Account:      "))
		s.WriteString(m.session.DisplayName())
		s.WriteString("\n")
	}
	if m.session.ARN != "" {
		s.WriteString(focusedStyle.Render("Identity:     "))
		s.WriteString(m.session.ARN)
		s.WriteString("\n")
	}
	if m.session.Region != "" {
		s.WriteString(focusedStyle.Render("Region:       "))
		s.WriteString("[")
		s.WriteString(m.session.Region)
		s.WriteString("]\n")
	}
	absPath, _ := filepath.Abs(m.cfg.CredentialsPath())
	s.WriteString(sourceStyle.Render("Config Path:  "))
	s.WriteString(absPath)
	s.WriteString("\n\n")
}

// writeBody renders the state-specific portion of the screen.
func (m Model) writeBody(s *strings.Builder) {
	switch m.state {
	case stateCheckingSession:
		s.WriteString("\n  ")
		s.WriteString(m.spinner.View())
		s.WriteString(" Verifying AWS Session...")
	case stateLogin:
		m.writeLogin(s)
	case stateSelectRegion, stateSelectEnv, stateSelectCluster, stateSelectService, stateSelectMethod:
		s.WriteString(m.selector.View())
	case stateMenu:
		s.WriteString(m.mainMenu.View())
	case stateLoading:
		s.WriteString("\n  ")
		s.WriteString(m.spinner.View())
		s.WriteString(" Fetching from AWS...")
	case stateInputs:
		m.writeInputs(s)
	case stateExecuting:
		s.WriteString(m.spinner.View())
		s.WriteString(" Executing ")
		s.WriteString(m.action)
		s.WriteString(" operation...")
	case stateResult:
		m.writeResult(s)
	}
}

func (m Model) writeLogin(s *strings.Builder) {
	s.WriteString(focusedStyle.Render("🔑 AWS Login Required"))
	s.WriteString("\n\n")
	s.WriteString(helpStyle.Render("Session missing or expired. Please provide your keys:"))
	s.WriteString("\n\n")

	// Suppress the expected "missing/expired" sentinels — only show real errors.
	if m.err != nil && !errors.Is(m.err, awsx.ErrCredentialsMissing) && !errors.Is(m.err, awsx.ErrCredentialsExpired) {
		s.WriteString(errorStyle.Render("Error: " + m.err.Error()))
		s.WriteString("\n\n")
	}
	m.writeInputFields(s)
}

func (m Model) writeInputs(s *strings.Builder) {
	s.WriteString(focusedStyle.Render("Confirm details:"))
	s.WriteString("\n\n")
	m.writeSummary(s)
	if len(m.inputs) == 0 {
		s.WriteString(helpStyle.Render("Press ENTER to confirm, or ESC to go back."))
		s.WriteString("\n")
		return
	}
	m.writeInputFields(s)
}

// writeSummary renders the operation context gathered across the preceding
// selection steps, so the confirmation screen is meaningful even when the chosen
// flow needs no further input (e.g. the task-definition retrieval method).
func (m Model) writeSummary(s *strings.Builder) {
	writeField := func(label, value string) {
		if value == "" {
			return
		}
		s.WriteString(focusedStyle.Render(label))
		s.WriteString(value)
		s.WriteString("\n")
	}
	writeField("Operation:    ", strings.ToUpper(m.action))
	writeField("Environment:  ", m.env)
	writeField("Cluster:      ", m.cluster)
	writeField("Service:      ", m.service)
	writeField("Method:       ", m.methodLabel())
	s.WriteString("\n")
}

// methodLabel maps the internal retrieval-method code to a human-readable label.
func (m Model) methodLabel() string {
	switch m.method {
	case "tdf":
		return "From Task Definition"
	case "path":
		return "By Path Prefix"
	}
	return ""
}

func (m Model) writeInputFields(s *strings.Builder) {
	for _, input := range m.inputs {
		s.WriteString(helpStyle.Render(input.Placeholder))
		s.WriteString("\n")
		s.WriteString(input.View())
		s.WriteString("\n\n")
	}
}

func (m Model) writeResult(s *strings.Builder) {
	if m.err != nil {
		s.WriteString(errorStyle.Render("Error: " + m.err.Error()))
		return
	}
	s.WriteString(successStyle.Render("Success!"))
	s.WriteString("\n\n")
	s.WriteString(m.result)
}

func (m Model) footerView() string {
	items := []string{keyStyle.Render("Q") + " Quit"}

	switch m.state {
	case stateMenu:
		items = append(items, keyStyle.Render("L")+" Login", keyStyle.Render("G")+" Region", keyStyle.Render("H")+" Help")
	case stateSelectEnv, stateSelectCluster, stateSelectService, stateSelectMethod:
		items = append(items, keyStyle.Render("ESC")+" Back", keyStyle.Render("/")+" Search", keyStyle.Render("H")+" Help")
	case stateInputs, stateLogin:
		esc := keyStyle.Render("ESC") + " Back"
		if m.state == stateLogin {
			esc = keyStyle.Render("ESC") + " Quit"
		}
		items = append(items, esc, keyStyle.Render("ENTER")+" Next/Save")
		if len(m.inputs) > 1 {
			items = append(items, keyStyle.Render("TAB")+" Move")
		}
	case stateResult:
		items = append(items, keyStyle.Render("ENTER")+" Menu")
	}
	return strings.Join(items, " • ")
}
