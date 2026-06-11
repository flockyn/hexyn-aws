package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
)

func TestSetupLoginInputsMasksSecrets(t *testing.T) {
	m := newTestModel(t)
	m.setupLoginInputs()

	if len(m.inputs) != 3 {
		t.Fatalf("expected 3 login inputs, got %d", len(m.inputs))
	}
	if m.focusIndex != 0 {
		t.Errorf("expected focus on first field, got %d", m.focusIndex)
	}
	if m.inputs[1].EchoMode != textinput.EchoPassword || m.inputs[2].EchoMode != textinput.EchoPassword {
		t.Error("secret key and session token should be masked")
	}
	if m.inputs[0].EchoMode == textinput.EchoPassword {
		t.Error("access key id should not be masked")
	}
}

func TestSetupInputsForPut(t *testing.T) {
	m := newTestModel(t)
	m.action = "put"
	m.service = "nft-service-orders" // exercises the repo-name cleaning

	m.setupInputs()

	if len(m.inputs) != 2 {
		t.Fatalf("expected 2 inputs for put, got %d", len(m.inputs))
	}
	if got := m.inputs[0].Value(); got != "orders" {
		t.Errorf("repo name not cleaned: got %q want %q", got, "orders")
	}
	if got := m.inputs[1].Value(); got != "orders.env" {
		t.Errorf("file name default wrong: got %q", got)
	}
}

func TestSetupInputsForGetPath(t *testing.T) {
	m := newTestModel(t)
	m.action = "get"
	m.method = "path"
	m.service = "service-users"

	m.setupInputs()

	// get/path: SSM Repo Name + Output Subdirectory Name.
	if len(m.inputs) != 2 {
		t.Fatalf("expected 2 inputs for get/path, got %d", len(m.inputs))
	}
	if got := m.inputs[0].Value(); got != "users" {
		t.Errorf("repo name not cleaned: got %q", got)
	}
	if got := m.inputs[len(m.inputs)-1].Placeholder; got != "Output Subdirectory Name" {
		t.Errorf("last input should be the output dir, got %q", got)
	}
}

func TestSetupInputsForGetTaskDefHasOutputDir(t *testing.T) {
	m := newTestModel(t)
	m.action = "get"
	m.method = "tdf"
	m.service = "service-users"

	m.setupInputs()

	// get/tdf: only the Output Subdirectory Name (the SSM path comes from the TDF).
	if len(m.inputs) != 1 {
		t.Fatalf("expected 1 input for get/tdf, got %d", len(m.inputs))
	}
	if got := m.inputs[0].Placeholder; got != "Output Subdirectory Name" {
		t.Errorf("expected output dir input, got %q", got)
	}
	if got := m.inputs[0].Value(); got != "users" {
		t.Errorf("output dir should default to cleaned service, got %q", got)
	}
}

func TestFocusNavigationCycles(t *testing.T) {
	m := newTestModel(t)
	m.setupLoginInputs() // 3 inputs, focus at 0

	m.nextInput()
	if m.focusIndex != 1 {
		t.Errorf("next: expected 1, got %d", m.focusIndex)
	}
	m.nextInput()
	m.nextInput() // wraps 2 -> 0
	if m.focusIndex != 0 {
		t.Errorf("next wrap: expected 0, got %d", m.focusIndex)
	}
	m.prevInput() // wraps 0 -> 2
	if m.focusIndex != 2 {
		t.Errorf("prev wrap: expected 2, got %d", m.focusIndex)
	}
}
