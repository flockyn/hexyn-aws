package envfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hexyn-aws/internal/awsx"
)

func TestParseLineKeyValue(t *testing.T) {
	p, ok := (FS{}).parseLine("FOO=bar")
	require.True(t, ok)
	assert.Equal(t, "FOO", p.Name)
	assert.Equal(t, "bar", p.Value)
	assert.False(t, p.IsSecure())
}

func TestParseLineSkipsBlankAndComment(t *testing.T) {
	for _, line := range []string{"", "   ", "# a comment"} {
		_, ok := (FS{}).parseLine(line)
		assert.Falsef(t, ok, "expected %q to be skipped", line)
	}
}

func TestParseLineMalformed(t *testing.T) {
	for _, line := range []string{"NOEQUALS", "=novalue"} {
		_, ok := (FS{}).parseLine(line)
		assert.Falsef(t, ok, "expected %q to be rejected", line)
	}
}

func TestParseLineSecureStringAnnotation(t *testing.T) {
	p, ok := (FS{}).parseLine("SECRET=s3cret //secureString")
	require.True(t, ok)
	assert.Equal(t, "s3cret", p.Value)
	assert.True(t, p.IsSecure())
}

func TestParseLinePreservesURLDoubleSlash(t *testing.T) {
	// Regression: a "//" inside the value must not be treated as a comment.
	cases := []struct{ line, value string }{
		{"RABBITMQ_URL=amqp://guest:guest@rabbitmq-5672-tcp:5672/", "amqp://guest:guest@rabbitmq-5672-tcp:5672/"},
		{"API_URL=https://api.example.com//v2/path", "https://api.example.com//v2/path"},
		{"REDIS_URL=redis://:pass@host:6379/0", "redis://:pass@host:6379/0"},
	}
	for _, tc := range cases {
		p, ok := (FS{}).parseLine(tc.line)
		require.Truef(t, ok, "%q: expected ok", tc.line)
		assert.Equalf(t, tc.value, p.Value, "%q", tc.line)
		assert.Falsef(t, p.IsSecure(), "%q: should not be SecureString", tc.line)
	}
}

func TestParseLineURLWithSecureMarker(t *testing.T) {
	// Full URL kept AND the trailing //secureString marker honoured.
	p, ok := (FS{}).parseLine("RABBITMQ_URL=amqp://guest:guest@rabbitmq-5672-tcp:5672/ //secureString")
	require.True(t, ok)
	assert.Equal(t, "amqp://guest:guest@rabbitmq-5672-tcp:5672/", p.Value)
	assert.Equal(t, awsx.ParameterTypeSecureString, p.Type)
}

func TestSecureMarkerIndex(t *testing.T) {
	cases := []struct {
		line string
		want bool // whether a marker is found
	}{
		{"amqp://guest:guest@host:5672/", false},
		{"plain value", false},
		{"secret //secureString", true},
		{"amqp://host //secureString", true},
		{"x // secureString", true}, // whitespace between // and securestring
	}
	for _, tc := range cases {
		got := (FS{}).secureMarkerIndex(tc.line) != -1
		assert.Equalf(t, tc.want, got, "secureMarkerIndex(%q)", tc.line)
	}
}

func TestUnwrapQuotes(t *testing.T) {
	cases := []struct{ in, want string }{
		{`'wrapped'`, "wrapped"},
		{`"wrapped"`, "wrapped"},
		{`'{"a":1}'`, `{"a":1}`},
		{"unquoted", "unquoted"},
		{`'mismatched"`, `'mismatched"`}, // different open/close quotes
		{`'only-leading`, `'only-leading`},
		{`trailing'`, `trailing'`},
		{"it's fine", "it's fine"}, // apostrophe inside, not a wrapper
		{"", ""},
		{`'`, `'`}, // single char, no pair
		{`''`, ""}, // empty wrapped
		{`""`, ""},
	}
	for _, tc := range cases {
		assert.Equalf(t, tc.want, (FS{}).unwrapQuotes(tc.in), "unwrapQuotes(%q)", tc.in)
	}
}

func TestJSONDepth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{`{"a":1}`, 0},
		{`[1,2,3]`, 0},
		{`{"a":`, 1},
		{`{"a":{"b":[`, 3},
		{`}}`, -2},
		{`{"t":"a{b}c"}`, 0},     // braces inside a string are ignored
		{`["a[b]c"]`, 0},         // brackets inside a string are ignored
		{`{"t":"a\"{b"}`, 0},     // escaped quote keeps string open
		{`{} //secureString`, 0}, // trailing marker has no braces
	}
	for _, tc := range cases {
		assert.Equalf(t, tc.want, (FS{}).jsonDepth(tc.in), "jsonDepth(%q)", tc.in)
	}
}

func TestJSONValuePart(t *testing.T) {
	cases := []struct {
		line   string
		want   string
		wantOK bool
	}{
		{`CFG={"a":1}`, `{"a":1}`, true},
		{`CFG=  {"a":1}`, `{"a":1}`, true},  // leading whitespace stripped
		{`CFG='{"a":1}'`, `{"a":1}'`, true}, // leading quote dropped, trailing kept
		{`CFG="{"a":1}"`, `{"a":1}"`, true}, // double quote variant
		{`CFG=[1,2]`, `[1,2]`, true},        // array opener
		{`CFG=plain`, "", false},            // not JSON
		{`CFG=`, "", false},                 // empty value
		{`noequals`, "", false},             // no "="
	}
	for _, tc := range cases {
		got, ok := (FS{}).jsonValuePart(tc.line)
		assert.Equalf(t, tc.wantOK, ok, "jsonValuePart(%q) ok", tc.line)
		assert.Equalf(t, tc.want, got, "jsonValuePart(%q) value", tc.line)
	}
}

func TestJSONValueOpens(t *testing.T) {
	assert.True(t, (FS{}).jsonValueOpens(`CFG={"a":`))    // unclosed object
	assert.True(t, (FS{}).jsonValueOpens(`CFG='{`))       // quoted, unclosed
	assert.False(t, (FS{}).jsonValueOpens(`CFG={"a":1}`)) // complete on one line
	assert.False(t, (FS{}).jsonValueOpens(`CFG=plain`))   // not JSON
}

func TestJSONValueComplete(t *testing.T) {
	assert.True(t, (FS{}).jsonValueComplete(`CFG={"a":1}`)) // balanced
	assert.True(t, (FS{}).jsonValueComplete(`CFG={"a":1}' //secureString`))
	assert.True(t, (FS{}).jsonValueComplete(`CFG=plain`))  // not JSON -> complete
	assert.False(t, (FS{}).jsonValueComplete(`CFG={"a":`)) // still open
}
