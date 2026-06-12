package envfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"hexyn-aws/internal/awsx"
)

// PEM block delimiters. A value opening with "-----BEGIN" spans every line up to
// the matching "-----END", so multi-line keys/certs survive parsing.
const (
	pemBegin = "-----BEGIN"
	pemEnd   = "-----END"
)

// Parse reads the given .env file into a slice of parameters. Most entries are a
// single KEY=VALUE line; a PEM block or a JSON object/array spanning several
// physical lines is reassembled into one parameter.
func (f FS) Parse(filePath string) ([]awsx.Parameter, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var params []awsx.Parameter
	add := func(entry string) {
		if p, ok := f.parseLine(entry); ok {
			params = append(params, p)
		}
	}

	var entry strings.Builder
	inPEM := false
	inJSON := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case inPEM:
			entry.WriteByte('\n')
			entry.WriteString(line)
			if strings.Contains(line, pemEnd) {
				inPEM = false
				add(entry.String())
				entry.Reset()
			}
		case inJSON:
			entry.WriteByte('\n')
			entry.WriteString(line)
			if f.jsonValueComplete(entry.String()) {
				inJSON = false
				add(entry.String())
				entry.Reset()
			}
		case strings.Contains(line, pemBegin):
			entry.WriteString(line)
			inPEM = true
		case f.jsonValueOpens(line):
			entry.WriteString(line)
			inJSON = true
		default:
			add(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	// Flush an unterminated multi-line value (PEM missing -----END, or JSON whose braces never balanced).
	if entry.Len() > 0 {
		add(entry.String())
	}
	return params, nil
}
