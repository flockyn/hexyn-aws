package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"hexyn-aws/internal/awsx"

	"github.com/charmbracelet/lipgloss"
)

// View renders the full screen, keeping the footer pinned to the bottom.
func (m Model) View() string {
	var s strings.Builder

	brand := titleStyle.MarginBottom(0).Render("HEXYN AWS CLI")
	if m.version != "" {
		brand += " " + sourceStyle.Render(m.version)
	}
	s.WriteString(brand)
	s.WriteString("\n\n")
	m.writeSessionHeader(&s)
	m.writeBody(&s)

	content := s.String()
	footer := m.footerView()

	if m.width > 0 {
		content = lipgloss.NewStyle().Width(m.width).Render(content)
		footer = lipgloss.NewStyle().Width(m.width).Render(footer)
	}

	if m.height == 0 { // size unknown yet (no WindowSizeMsg) — fall back to flow layout
		return appStyle.Render(content + "\n\n" + footer)
	}
	// Pad so the footer sits on the last row(s); +1 turns the row gap into newlines.
	gap := max(m.height-lipgloss.Height(content)-lipgloss.Height(footer)+1, 1)
	return appStyle.Render(content + strings.Repeat("\n", gap) + footer)
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
	case stateSelectRegion, stateSelectCluster, stateSelectService, stateSelectMethod:
		s.WriteString(m.selector.View())
	case stateMenu:
		s.WriteString(m.mainMenu.View())
	case stateLoading:
		s.WriteString("\n  ")
		s.WriteString(m.spinner.View())
		s.WriteString(" Fetching from AWS...")
	case stateInputs:
		m.writeInputs(s)
	case stateConfig:
		m.writeConfig(s)
	case stateConfirmPut:
		m.writeConfirmPut(s)
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

// writeConfig renders the settings screen: the editable repo-prefix field plus
// where it persists.
func (m Model) writeConfig(s *strings.Builder) {
	s.WriteString(focusedStyle.Render("Settings"))
	s.WriteString("\n\n")
	s.WriteString(helpStyle.Render("Service-name prefixes stripped to derive the SSM repo name."))
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("Saved to " + m.cfg.ConfigPath() + " (the HEXYN_REPO_PREFIXES env var overrides it)."))
	s.WriteString("\n\n")
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

// confirmValueWidth is the fallback wrap width for the confirmation table's value
// column when the terminal width is not yet known.
const confirmValueWidth = 60

// writeConfirmPut renders the pre-upload review: the operation summary, the
// destination path, and an ENV/VALUE table of every parameter that will be sent.
// Long values wrap onto continuation lines aligned under the VALUE column instead
// of being truncated, so the full value is reviewable.
func (m Model) writeConfirmPut(s *strings.Builder) {
	s.WriteString(focusedStyle.Render("Confirm upload — review parameters:"))
	s.WriteString("\n\n")
	m.writeSummary(s)
	s.WriteString(focusedStyle.Render("Destination:  "))
	fmt.Fprintf(s, "/%s/%s/", m.inputs[0].Value(), m.inputs[1].Value())
	s.WriteString("\n\n")

	if len(m.previewParams) == 0 {
		s.WriteString(errorStyle.Render("No parameters found in the selected file."))
		s.WriteString("\n")
		return
	}

	nameWidth := len("ENV")
	for _, p := range m.previewParams {
		if len(p.Name) > nameWidth {
			nameWidth = len(p.Name)
		}
	}
	valueWidth := confirmValueWidth
	if m.width > 0 {
		valueWidth = max(m.width-nameWidth-2, 20)
	}
	indent := strings.Repeat(" ", nameWidth+2)

	s.WriteString(focusedStyle.Render(fmt.Sprintf("%-*s  %s", nameWidth, "ENV", "VALUE")))
	s.WriteString("\n")
	for _, p := range m.previewParams {
		lines := m.wrapValue(p.Value, valueWidth)
		if p.IsSecure() {
			lines[len(lines)-1] += helpStyle.Render(" //secureString")
		}
		fmt.Fprintf(s, "%-*s  %s\n", nameWidth, p.Name, lines[0])
		for _, cont := range lines[1:] {
			fmt.Fprintf(s, "%s%s\n", indent, cont)
		}
	}
	s.WriteString("\n")
	s.WriteString(helpStyle.Render(fmt.Sprintf("%d parameter(s) will be uploaded. Press ENTER to confirm, ESC to go back.", len(m.previewParams))))
	s.WriteString("\n")
}

// wrapValue renders a value as one or more display lines no wider than width,
// flattening embedded newlines first. The full value is still uploaded; only the
// on-screen layout is wrapped. The receiver is unused; it keeps the helper grouped
// on Model.
func (Model) wrapValue(v string, width int) []string {
	v = strings.ReplaceAll(v, "\n", "\\n")
	if width <= 0 {
		return []string{v}
	}
	runes := []rune(v)
	var lines []string
	for len(runes) > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	return append(lines, string(runes)) // final chunk (empty string for an empty value)
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
	case stateSelectCluster, stateSelectService, stateSelectMethod:
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
	case stateConfig:
		items = append(items, keyStyle.Render("ESC")+" Back", keyStyle.Render("ENTER")+" Save")
	case stateConfirmPut:
		items = append(items, keyStyle.Render("ESC")+" Back", keyStyle.Render("ENTER")+" Confirm Upload")
	case stateResult:
		items = append(items, keyStyle.Render("ENTER")+" Menu")
	}
	if m.settingsAvailable() { // global shortcut, shown on every interactive screen
		items = append(items, keyStyle.Render("S")+" Settings")
	}
	return strings.Join(items, " • ")
}
