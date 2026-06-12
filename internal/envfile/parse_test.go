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

// TestParseSingleLineJSON verifies a compact JSON value on one line round-trips,
// including embedded "=" (base64 padding) and "//" (a URL) inside strings, which
// must not be mistaken for a KEY split or the //secureString marker.
func TestParseSingleLineJSON(t *testing.T) {
	body := `CONFIG={"url":"http://h/x","token":"abc==","nested":{"k":[1,2]}}` + "\n"
	got, err := FS{}.Parse(fixtures.WriteTemp(t, body))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "CONFIG", got[0].Name)
	assert.Equal(t, `{"url":"http://h/x","token":"abc==","nested":{"k":[1,2]}}`, got[0].Value)
}

// TestParseMultilineJSONObject reassembles a pretty-printed JSON object spanning
// several physical lines into a single parameter, preserving the surrounding
// single-line entries.
func TestParseMultilineJSONObject(t *testing.T) {
	value := "{\n  \"host\": \"db\",\n  \"ports\": [5432, 5433],\n  \"opts\": {\"ssl\": true}\n}"
	body := "BEFORE=a\n" + "CONFIG=" + value + "\n" + "AFTER=b\n"
	got, err := FS{}.Parse(fixtures.WriteTemp(t, body))
	require.NoError(t, err)
	require.Len(t, got, 3)

	byName := map[string]string{}
	for _, p := range got {
		byName[p.Name] = p.Value
	}
	assert.Equal(t, "a", byName["BEFORE"])
	assert.Equal(t, "b", byName["AFTER"])
	assert.Equal(t, value, byName["CONFIG"])
}

// TestParseMultilineJSONArray reassembles a multi-line JSON array value.
func TestParseMultilineJSONArray(t *testing.T) {
	value := "[\n  \"one\",\n  \"two\"\n]"
	got, err := FS{}.Parse(fixtures.WriteTemp(t, "LIST="+value+"\n"))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "LIST", got[0].Name)
	assert.Equal(t, value, got[0].Value)
}

// TestParseMultilineJSONBraceInString ensures braces inside a JSON string do not
// prematurely close (or fail to close) the value.
func TestParseMultilineJSONBraceInString(t *testing.T) {
	value := "{\n  \"tmpl\": \"a{b}c\",\n  \"x\": 1\n}"
	got, err := FS{}.Parse(fixtures.WriteTemp(t, "T="+value+"\n"))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, value, got[0].Value)
}

// TestParseMultilineJSONSecureMarker reassembles a multi-line JSON value whose
// closing line carries the //secureString marker, which must be stripped and the
// parameter flagged SecureString.
func TestParseMultilineJSONSecureMarker(t *testing.T) {
	value := "{\n  \"k\": \"v\"\n}"
	body := "SECRET=" + value + " //secureString\n"
	got, err := FS{}.Parse(fixtures.WriteTemp(t, body))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "SECRET", got[0].Name)
	assert.Equal(t, value, got[0].Value)
	assert.True(t, got[0].IsSecure(), "expected JSON param to be SecureString")
}

// TestParseSingleLineQuotedJSON verifies that a shell-quoted JSON value has its
// wrapping quotes stripped, for both single and double quotes.
func TestParseSingleLineQuotedJSON(t *testing.T) {
	cases := map[string]string{
		"SINGLE=" + `'{"foo": "bar", "baz": true}'`: `{"foo": "bar", "baz": true}`,
		"DOUBLE=" + `"{"foo":"bar"}"`:               `{"foo":"bar"}`,
	}
	for line, want := range cases {
		got, err := FS{}.Parse(fixtures.WriteTemp(t, line+"\n"))
		require.NoError(t, err, line)
		require.Len(t, got, 1, line)
		assert.Equal(t, want, got[0].Value, line)
	}
}

// TestParseMultilineQuotedJSON reassembles a multi-line JSON value wrapped in
// single quotes, stripping the wrapper from the stored value.
func TestParseMultilineQuotedJSON(t *testing.T) {
	inner := "{\n  \"foo\": \"bar\",\n  \"baz\": true\n}"
	body := "CONFIG='" + inner + "'\n"
	got, err := FS{}.Parse(fixtures.WriteTemp(t, body))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "CONFIG", got[0].Name)
	assert.Equal(t, inner, got[0].Value)
}

// TestParseUnwrapPreservesUnmatched ensures values that are not a matching
// quote pair keep their quotes (e.g. an apostrophe in a plain value).
func TestParseUnwrapPreservesUnmatched(t *testing.T) {
	got, err := FS{}.Parse(fixtures.WriteTemp(t, "MSG=it's fine\n"))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "it's fine", got[0].Value)
}
