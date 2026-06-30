package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupLoginInputsMasksSecrets(t *testing.T) {
	m := newTestModel(t)
	m.setupLoginInputs()

	require.Len(t, m.inputs, 3)
	assert.Equal(t, 0, m.focusIndex, "expected focus on first field")
	assert.Equal(t, textinput.EchoPassword, m.inputs[1].EchoMode, "secret key should be masked")
	assert.Equal(t, textinput.EchoPassword, m.inputs[2].EchoMode, "session token should be masked")
	assert.NotEqual(t, textinput.EchoPassword, m.inputs[0].EchoMode, "access key id should not be masked")
}

func TestSetupInputsForPut(t *testing.T) {
	t.Setenv("HEXYN_REPO_PREFIXES", "service-") // exercise repo-name cleaning
	m := newTestModel(t)
	m.action = "put"
	m.service = "service-orders"

	m.setupInputs()

	require.Len(t, m.inputs, 3)
	assert.Equal(t, "orders", m.inputs[1].Value(), "repo name not cleaned")
	assert.Equal(t, "orders.env", m.inputs[2].Value(), "file name default wrong")
}

func TestSetupInputsForGetPath(t *testing.T) {
	t.Setenv("HEXYN_REPO_PREFIXES", "service-")
	m := newTestModel(t)
	m.action = "get"
	m.method = "path"
	m.service = "service-users"

	m.setupInputs()

	// get/path: Environment + SSM Repo Name + Output Subdirectory Name.
	require.Len(t, m.inputs, 3)
	assert.Equal(t, "users", m.inputs[1].Value(), "repo name not cleaned")
	assert.Equal(t, "Output Subdirectory Name", m.inputs[len(m.inputs)-1].Placeholder, "last input should be the output dir")
}

func TestSetupInputsForGetTaskDefHasOutputDir(t *testing.T) {
	t.Setenv("HEXYN_REPO_PREFIXES", "service-")
	m := newTestModel(t)
	m.action = "get"
	m.method = "tdf"
	m.service = "service-users"

	m.setupInputs()

	// get/tdf: Environment + Output Subdirectory Name (the SSM path comes from the TDF).
	require.Len(t, m.inputs, 2)
	assert.Equal(t, "Output Subdirectory Name", m.inputs[1].Placeholder, "expected output dir input")
	assert.Equal(t, "users", m.inputs[1].Value(), "output dir should default to cleaned service")
}

func TestFocusNavigationCycles(t *testing.T) {
	m := newTestModel(t)
	m.setupLoginInputs() // 3 inputs, focus at 0

	m.nextInput()
	assert.Equal(t, 1, m.focusIndex, "next")
	m.nextInput()
	m.nextInput() // wraps 2 -> 0
	assert.Equal(t, 0, m.focusIndex, "next wrap")
	m.prevInput() // wraps 0 -> 2
	assert.Equal(t, 2, m.focusIndex, "prev wrap")
}
