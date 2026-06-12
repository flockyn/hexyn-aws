package awsx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParameterIsSecure(t *testing.T) {
	assert.True(t, Parameter{Type: ParameterTypeSecureString}.IsSecure(), "SecureString parameter should report IsSecure() == true")
	assert.False(t, Parameter{Type: ParameterTypeString}.IsSecure(), "String parameter should report IsSecure() == false")
}

func TestTaskSecretPath(t *testing.T) {
	arn := TaskSecret{SSMPath: "arn:aws:ssm:ap-southeast-3:123456789012:parameter/prod/api/DB_HOST"}
	assert.Equal(t, "/prod/api/DB_HOST", arn.Path(), "ARN prefix should be stripped to the bare path")

	bare := TaskSecret{SSMPath: "/prod/api/DB_HOST"}
	assert.Equal(t, "/prod/api/DB_HOST", bare.Path(), "a bare path should be returned unchanged")
}

func TestSessionDisplayName(t *testing.T) {
	withAlias := Session{AccountID: "123456789012", AccountAlias: "prod"}
	assert.Equal(t, "prod", withAlias.DisplayName(), "expected alias to win")

	noAlias := Session{AccountID: "123456789012"}
	assert.Equal(t, "123456789012", noAlias.DisplayName(), "expected account id fallback")
}
