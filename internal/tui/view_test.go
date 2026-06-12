package tui

import (
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"hexyn-aws/internal/awsx"
)

func TestViewAlwaysRendersTitle(t *testing.T) {
	assert.Contains(t, newTestModel(t).View(), "HEXYN AWS CLI", "expected the title in every view")
}

func TestViewShowsVersionNextToBrand(t *testing.T) {
	assert.Contains(t, newTestModel(t).View(), "v-test", "expected the version shown next to the brand label")
}

func TestViewPinsFooterToBottom(t *testing.T) {
	m := newTestModel(t)
	m.state = stateMenu
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	out := updated.(Model).View()

	assert.Equal(t, 24, lipgloss.Height(out), "view should fill the screen height so the footer sits at the bottom")
}

func TestWrapValueChunksToWidth(t *testing.T) {
	m := newTestModel(t)

	assert.Equal(t, []string{"short"}, m.wrapValue("short", 10), "a short value stays on one line")
	assert.Equal(t, []string{"abcde", "fghij", "k"}, m.wrapValue("abcdefghijk", 5), "a long value wraps into width-sized chunks")
	assert.Equal(t, []string{`a\nb`}, m.wrapValue("a\nb", 10), "embedded newlines are flattened, not wrapped")
	assert.Equal(t, []string{""}, m.wrapValue("", 10), "an empty value yields a single empty line")
}

func TestConfirmPutWrapsLongValues(t *testing.T) {
	m := newTestModel(t)
	m.action = "put"
	m.env = "prod"
	m.setupInputs()
	m.inputs[0].SetValue("api")
	m.state = stateConfirmPut
	m.previewParams = []awsx.Parameter{{Name: "TOKEN", Value: "0123456789abcdefghij0123456789abcdefghij0123456789abcdefghij0123456789"}}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	out := updated.(Model).View()

	assert.NotContains(t, out, "…", "long values should wrap, not be truncated")
	assert.Contains(t, out, "0123456789abcdefghij", "the full value is shown across wrapped lines")
}

func TestViewLoginPrompt(t *testing.T) {
	m := newTestModel(t)
	m.state = stateLogin
	m.setupLoginInputs()

	assert.Contains(t, m.View(), "AWS Login Required", "login view should prompt for credentials")
}

func TestViewResultSuccessAndError(t *testing.T) {
	m := newTestModel(t)
	m.state = stateResult

	m.result = "exported 5 params"
	out := m.View()
	assert.Contains(t, out, "Success!")
	assert.Contains(t, out, "exported 5 params")

	m.result = ""
	m.err = errors.New("boom")
	out = m.View()
	assert.Contains(t, out, "Error:")
	assert.Contains(t, out, "boom")
}

func TestViewConfirmTaskDefMethodShowsSummary(t *testing.T) {
	m := newTestModel(t)
	m.state = stateSelectService
	m.action = "get"
	m.env = "prod"
	m.cluster = "prod-cluster"
	m.service = "service-api"
	m.selector.SetItems([]list.Item{item{title: "service-api"}})
	m.selector.Select(0)

	updated, _ := m.selectCurrent()
	mu := updated.(Model)
	mu.method = "tdf"
	mu.setupInputs()
	mu.state = stateInputs

	out := mu.View()
	for _, want := range []string{"Confirm details:", "GET", "prod", "prod-cluster", "service-api", "From Task Definition", "Output Subdirectory Name"} {
		assert.Containsf(t, out, want, "tdf confirmation missing %q", want)
	}
}

func TestViewConfirmPathMethodShowsSummaryAndInput(t *testing.T) {
	m := newTestModel(t)
	m.action = "get"
	m.env = "preprod"
	m.cluster = "c1"
	m.service = "service-billing"
	m.method = "path"
	m.setupInputs()
	m.state = stateInputs

	out := m.View()
	for _, want := range []string{"By Path Prefix", "preprod", "SSM Repo Name"} {
		assert.Containsf(t, out, want, "path confirmation missing %q", want)
	}
}

func TestFooterMenuHints(t *testing.T) {
	m := newTestModel(t)
	m.state = stateMenu

	footer := m.footerView()
	for _, want := range []string{"Quit", "Login", "Region", "Help"} {
		assert.Containsf(t, footer, want, "menu footer missing %q", want)
	}
}
