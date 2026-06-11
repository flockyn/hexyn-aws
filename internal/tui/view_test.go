package tui

import (
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/stretchr/testify/assert"
)

func TestViewAlwaysRendersTitle(t *testing.T) {
	assert.Contains(t, newTestModel(t).View(), "HEXYN AWS CLI", "expected the title in every view")
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
	m.service = "nft-service-api"
	m.selector.SetItems([]list.Item{item{title: "nft-service-api"}})
	m.selector.Select(0)

	updated, _ := m.selectCurrent()
	mu := updated.(Model)
	mu.method = "tdf"
	mu.setupInputs()
	mu.state = stateInputs

	out := mu.View()
	for _, want := range []string{"Confirm details:", "GET", "prod", "prod-cluster", "nft-service-api", "From Task Definition", "Output Subdirectory Name"} {
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
