package tui

import (
	"testing"

	"hexyn-aws/internal/config"
	"hexyn-aws/internal/secrets"
	mocks "hexyn-aws/test/mocks/secrets"
)

// newTestModel builds a Model with a nil service, sufficient for the state
// transition and view tests that never reach a service call.
func newTestModel(t *testing.T) Model {
	t.Helper()
	return NewModel(nil, config.New(true), "v-test") // svc unused by the transitions under test
}

// runnerWith builds a commandRunner over a real Service, defaulting any
// dependency the test does not exercise to an empty mock.
func runnerWith(t *testing.T, d secrets.Deps) *commandRunner {
	if d.Cfg == nil {
		d.Cfg = config.New(true)
	}
	if d.Creds == nil {
		d.Creds = mocks.NewMockCredentialStore(t)
	}
	if d.Session == nil {
		d.Session = mocks.NewMockSessionClient(t)
	}
	if d.Env == nil {
		d.Env = mocks.NewMockEnvFiles(t)
	}
	if d.AWS == nil {
		d.AWS = mocks.NewMockAWS(t)
	}
	return &commandRunner{svc: secrets.New(d)}
}
