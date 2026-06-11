package envfile

import (
	"testing"

	"hexyn-aws/internal/awsx"
)

func TestParseLineKeyValue(t *testing.T) {
	p, ok := (FS{}).parseLine("FOO=bar")
	if !ok {
		t.Fatal("expected ok")
	}
	if p.Name != "FOO" || p.Value != "bar" || p.IsSecure() {
		t.Errorf("unexpected parse: %+v", p)
	}
}

func TestParseLineSkipsBlankAndComment(t *testing.T) {
	for _, line := range []string{"", "   ", "# a comment"} {
		if _, ok := (FS{}).parseLine(line); ok {
			t.Errorf("expected %q to be skipped", line)
		}
	}
}

func TestParseLineMalformed(t *testing.T) {
	for _, line := range []string{"NOEQUALS", "=novalue"} {
		if _, ok := (FS{}).parseLine(line); ok {
			t.Errorf("expected %q to be rejected", line)
		}
	}
}

func TestParseLineSecureStringAnnotation(t *testing.T) {
	p, ok := (FS{}).parseLine("SECRET=s3cret //secureString")
	if !ok {
		t.Fatal("expected ok")
	}
	if p.Value != "s3cret" {
		t.Errorf("inline marker not stripped: %q", p.Value)
	}
	if !p.IsSecure() {
		t.Error("expected SecureString")
	}
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
		if !ok {
			t.Fatalf("%q: expected ok", tc.line)
		}
		if p.Value != tc.value {
			t.Errorf("%q: got value %q, want %q", tc.line, p.Value, tc.value)
		}
		if p.IsSecure() {
			t.Errorf("%q: should not be SecureString", tc.line)
		}
	}
}

func TestParseLineURLWithSecureMarker(t *testing.T) {
	// Full URL kept AND the trailing //secureString marker honoured.
	p, ok := (FS{}).parseLine("RABBITMQ_URL=amqp://guest:guest@rabbitmq-5672-tcp:5672/ //secureString")
	if !ok {
		t.Fatal("expected ok")
	}
	if p.Value != "amqp://guest:guest@rabbitmq-5672-tcp:5672/" {
		t.Errorf("URL truncated: %q", p.Value)
	}
	if p.Type != awsx.ParameterTypeSecureString {
		t.Error("expected SecureString")
	}
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
		if got != tc.want {
			t.Errorf("secureMarkerIndex(%q) found=%v, want %v", tc.line, got, tc.want)
		}
	}
}
