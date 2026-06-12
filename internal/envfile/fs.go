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
	value := f.unwrapQuotes(strings.TrimSpace(parts[1]))
	return awsx.Parameter{Name: key, Value: value, Type: paramType}, true
}

// unwrapQuotes removes one matching pair of surrounding single or double quotes,
// so a shell-quoted value (e.g. CONFIG='{"foo":"bar"}') does not keep its
// wrapping quotes. Inner text, and unmatched or single quotes, are left as-is.
func (FS) unwrapQuotes(s string) string {
	if len(s) >= 2 {
		if c := s[0]; (c == '\'' || c == '"') && s[len(s)-1] == c {
			return s[1 : len(s)-1]
		}
	}
	return s
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

// jsonValueOpens reports whether line's value begins a JSON object or array that
// is not yet closed, so the following lines belong to the same value.
func (f FS) jsonValueOpens(line string) bool {
	value, ok := f.jsonValuePart(line)
	return ok && f.jsonDepth(value) > 0
}

// jsonValueComplete reports whether the accumulated JSON value has balanced its
// braces and brackets. A value that is no longer JSON is treated as complete.
func (f FS) jsonValueComplete(entry string) bool {
	value, ok := f.jsonValuePart(entry)
	if !ok {
		return true
	}
	return f.jsonDepth(value) <= 0
}

// jsonValuePart returns the JSON value side of a "KEY=VALUE" entry, ok=false if it
// does not open with "{" or "[". Leading whitespace and a single wrapping quote
// ('...' or "...") are stripped so shell-quoted values are recognised.
func (FS) jsonValuePart(s string) (string, bool) {
	_, after, found := strings.Cut(s, "=")
	if !found {
		return "", false
	}
	value := strings.TrimLeft(after, " \t")
	value = strings.TrimPrefix(value, "'")
	value = strings.TrimPrefix(value, `"`)
	if value == "" || (value[0] != '{' && value[0] != '[') {
		return "", false
	}
	return value, true
}

// jsonDepth returns the net brace/bracket nesting depth of s. Characters inside
// double-quoted strings (honouring backslash escapes) are ignored, so braces
// within a string value do not skew the count.
func (FS) jsonDepth(s string) int {
	depth := 0
	inString := false
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// literal character inside a JSON string
		case r == '{', r == '[':
			depth++
		case r == '}', r == ']':
			depth--
		}
	}
	return depth
}
