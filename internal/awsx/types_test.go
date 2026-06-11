package awsx

import "testing"

func TestParameterIsSecure(t *testing.T) {
	if !(Parameter{Type: ParameterTypeSecureString}).IsSecure() {
		t.Error("SecureString parameter should report IsSecure() == true")
	}
	if (Parameter{Type: ParameterTypeString}).IsSecure() {
		t.Error("String parameter should report IsSecure() == false")
	}
}

func TestSessionDisplayName(t *testing.T) {
	withAlias := Session{AccountID: "123456789012", AccountAlias: "prod"}
	if got := withAlias.DisplayName(); got != "prod" {
		t.Errorf("expected alias to win, got %q", got)
	}

	noAlias := Session{AccountID: "123456789012"}
	if got := noAlias.DisplayName(); got != "123456789012" {
		t.Errorf("expected account id fallback, got %q", got)
	}
}
