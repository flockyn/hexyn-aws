package envfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"hexyn-aws/internal/awsx"
)

// PEM block delimiters. A value that opens with a "-----BEGIN" marker spans
// every following physical line up to and including the matching "-----END"
// line, so multi-line keys/certs (e.g. an RSA public key) survive parsing.
const (
	pemBegin = "-----BEGIN"
	pemEnd   = "-----END"
)

// Parse reads the given .env file into a slice of parameters. Most entries are a
// single KEY=VALUE line, but a value containing a PEM block is reassembled from
// its multiple physical lines so the key is preserved intact.
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
		case strings.Contains(line, pemBegin):
			entry.WriteString(line)
			inPEM = true
		default:
			add(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	// Flush an unterminated PEM block (missing -----END marker).
	if entry.Len() > 0 {
		add(entry.String())
	}
	return params, nil
}
