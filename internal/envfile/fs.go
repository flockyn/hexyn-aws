// Package envfile reads and writes the KEY=VALUE .env files the tool exchanges
// with SSM. A trailing " //secureString" comment marks a SecureString parameter.
package envfile

import (
	"strings"

	"hexyn-aws/internal/awsx"
)

// FS reads and writes .env files on the local filesystem. It is stateless and
// exists to provide the method set required by the secrets.EnvFiles port
// (Parse / Write / WriteRaw, implemented in their respective files).
type FS struct{}

// parseLine turns one .env line into a Parameter, reporting ok=false for blank
// lines, comments, and malformed entries.
func (f FS) parseLine(raw string) (awsx.Parameter, bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return awsx.Parameter{}, false
	}

	// Only the "//secureString" marker is a comment; a bare "//" inside a value
	// (e.g. amqp://, http://, redis://) must be preserved.
	isSecure := false
	if idx := f.secureMarkerIndex(line); idx != -1 {
		isSecure = true
		line = strings.TrimSpace(line[:idx])
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return awsx.Parameter{}, false
	}
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return awsx.Parameter{}, false
	}

	paramType := awsx.ParameterTypeString
	if isSecure {
		paramType = awsx.ParameterTypeSecureString
	}
	return awsx.Parameter{Name: key, Value: strings.TrimSpace(parts[1]), Type: paramType}, true
}

// secureMarkerIndex returns the index of the "//secureString" comment marker, or
// -1 if absent. It only matches a "//" that is directly followed (ignoring
// surrounding whitespace) by "securestring", so a "//" inside a value such as
// amqp://host is not mistaken for the marker.
func (FS) secureMarkerIndex(line string) int {
	lower := strings.ToLower(line)
	for from := 0; ; {
		rel := strings.Index(lower[from:], "//")
		if rel == -1 {
			return -1
		}
		idx := from + rel
		if strings.HasPrefix(strings.TrimSpace(lower[idx+2:]), "securestring") {
			return idx
		}
		from = idx + 2
	}
}
