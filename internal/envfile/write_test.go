package envfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hexyn-aws/internal/awsx"
	"hexyn-aws/test/fixtures"
)

func TestWriteSortsAndAnnotates(t *testing.T) {
	dir := t.TempDir()
	params := []awsx.Parameter{
		{Name: "ZED", Value: "last"},
		{Name: "API_KEY", Value: "secret", Type: awsx.ParameterTypeSecureString},
		{Name: "ALPHA", Value: "first"},
	}

	require.NoError(t, (FS{}).Write(dir, "out.env", params))

	data, err := os.ReadFile(filepath.Join(dir, "out.env"))
	require.NoError(t, err)
	want := "ALPHA=first\nAPI_KEY=secret //secureString\nZED=last\n"
	assert.Equal(t, want, string(data))
}

// TestWriteParseRoundTripURL verifies Write produces output that Parse reads back
// losslessly — including a secure URL whose value contains "//".
func TestWriteParseRoundTripURL(t *testing.T) {
	dir := t.TempDir()
	in := []awsx.Parameter{
		{Name: "RABBITMQ_URL", Value: "amqp://guest:guest@rabbitmq-5672-tcp:5672/", Type: awsx.ParameterTypeSecureString},
		{Name: "PLAIN_URL", Value: "http://example.com//x", Type: awsx.ParameterTypeString},
	}
	require.NoError(t, (FS{}).Write(dir, "rt.env", in))
	out, err := FS{}.Parse(filepath.Join(dir, "rt.env"))
	require.NoError(t, err)

	byName := map[string]awsx.Parameter{}
	for _, p := range out {
		byName[p.Name] = p
	}
	got := byName["RABBITMQ_URL"]
	assert.Equal(t, in[0].Value, got.Value)
	assert.True(t, got.IsSecure())

	got = byName["PLAIN_URL"]
	assert.Equal(t, in[1].Value, got.Value)
	assert.False(t, got.IsSecure())
}

// TestWriteParseRoundTripPEM verifies a multi-line PEM secure value survives a
// Write → Parse cycle unchanged.
func TestWriteParseRoundTripPEM(t *testing.T) {
	dir := t.TempDir()
	in := []awsx.Parameter{
		{Name: "ENCRYPTION_PUBLIC_KEY", Value: fixtures.PEMPublicKey, Type: awsx.ParameterTypeSecureString},
		{Name: "PLAIN", Value: "value", Type: awsx.ParameterTypeString},
	}
	require.NoError(t, (FS{}).Write(dir, "pem.env", in))
	out, err := FS{}.Parse(filepath.Join(dir, "pem.env"))
	require.NoError(t, err)

	byName := map[string]awsx.Parameter{}
	for _, p := range out {
		byName[p.Name] = p
	}
	got := byName["ENCRYPTION_PUBLIC_KEY"]
	assert.Equal(t, fixtures.PEMPublicKey, got.Value)
	assert.True(t, got.IsSecure())

	got = byName["PLAIN"]
	assert.Equal(t, "value", got.Value)
	assert.False(t, got.IsSecure())
}
