package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
)

func TestViewAlwaysRendersTitle(t *testing.T) {
	if !strings.Contains(newTestModel(t).View(), "HEXYN AWS CLI") {
		t.Error("expected the title in every view")
	}
}

func TestViewLoginPrompt(t *testing.T) {
	m := newTestModel(t)
	m.state = stateLogin
	m.setupLoginInputs()

	if !strings.Contains(m.View(), "AWS Login Required") {
		t.Error("login view should prompt for credentials")
	}
}

func TestViewResultSuccessAndError(t *testing.T) {
	m := newTestModel(t)
	m.state = stateResult

	m.result = "exported 5 params"
	if out := m.View(); !strings.Contains(out, "Success!") || !strings.Contains(out, "exported 5 params") {
		t.Errorf("success view missing message: %q", out)
	}

	m.result = ""
	m.err = errors.New("boom")
	if out := m.View(); !strings.Contains(out, "Error:") || !strings.Contains(out, "boom") {
		t.Errorf("error view missing error: %q", out)
	}
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
		if !strings.Contains(out, want) {
			t.Errorf("tdf confirmation missing %q in:\n%s", want, out)
		}
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
		if !strings.Contains(out, want) {
			t.Errorf("path confirmation missing %q in:\n%s", want, out)
		}
	}
}

func TestFooterMenuHints(t *testing.T) {
	m := newTestModel(t)
	m.state = stateMenu

	footer := m.footerView()
	for _, want := range []string{"Quit", "Login", "Region", "Help"} {
		if !strings.Contains(footer, want) {
			t.Errorf("menu footer missing %q: %s", want, footer)
		}
	}
}
