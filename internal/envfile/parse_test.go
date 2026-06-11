package envfile

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hexyn-aws/test/fixtures"
)

func TestParseReadsAllPairs(t *testing.T) {
	path := fixtures.WriteTemp(t, "FOO=bar\nBAZ=qux\n")

	got, err := FS{}.Parse(path)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "FOO", got[0].Name)
	assert.Equal(t, "bar", got[0].Value)
}

func TestParseSkipsBlankAndCommentLines(t *testing.T) {
	path := fixtures.WriteTemp(t, "\n# a comment\nFOO=bar\n   \n")

	got, err := FS{}.Parse(path)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestParseMissingFile(t *testing.T) {
	_, err := (FS{}).Parse(filepath.Join(t.TempDir(), "nope.env"))
	assert.Error(t, err)
}

// TestParseMultilinePEM verifies a PEM block spanning many physical lines is
// reassembled into a single parameter, with the trailing //secureString marker
// stripped, even when neighbouring single-line entries surround it.
func TestParseMultilinePEM(t *testing.T) {
	body := "BEFORE=a\n" +
		"ENCRYPTION_PUBLIC_KEY=" + fixtures.PEMPublicKey + " //secureString\n" +
		"AFTER=b\n"
	path := fixtures.WriteTemp(t, body)

	got, err := FS{}.Parse(path)
	require.NoError(t, err)
	require.Len(t, got, 3)

	byName := map[string]string{}
	secure := map[string]bool{}
	for _, p := range got {
		byName[p.Name] = p.Value
		secure[p.Name] = p.IsSecure()
	}
	assert.Equal(t, "a", byName["BEFORE"])
	assert.Equal(t, "b", byName["AFTER"])
	assert.Equal(t, fixtures.PEMPublicKey, byName["ENCRYPTION_PUBLIC_KEY"])
	assert.True(t, secure["ENCRYPTION_PUBLIC_KEY"], "expected PEM param to be SecureString")
}

// TestParseUnterminatedPEM ensures a PEM block missing its END marker is still
// flushed at EOF rather than silently dropped.
func TestParseUnterminatedPEM(t *testing.T) {
	body := "KEY=-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkq\n"
	got, err := FS{}.Parse(fixtures.WriteTemp(t, body))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "KEY", got[0].Name)
	assert.Equal(t, "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkq", got[0].Value)
}
