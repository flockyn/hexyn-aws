package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Parameter represents a single SSM parameter
type Parameter struct {
	Name  string
	Value string
	Type  string // "String" or "SecureString"
}

// ParseEnvFile robustly parses a .env file and extracts parameters.
// It supports "//secureString" comments to mark a parameter as SecureString.
func ParseEnvFile(filePath string) ([]Parameter, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var parameters []Parameter
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and full-line comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle trailing comments and identify SecureString
		isSecure := false
		if idx := strings.Index(line, "//"); idx != -1 {
			comment := strings.ToLower(line[idx+2:])
			if strings.Contains(comment, "securestring") {
				isSecure = true
			}
			line = strings.TrimSpace(line[:idx])
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Skip lines that don't look like KEY=VALUE
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Basic validation: key shouldn't be empty
		if key == "" {
			continue
		}

		paramType := "String"
		if isSecure {
			paramType = "SecureString"
		}

		parameters = append(parameters, Parameter{
			Name:  key,
			Value: value,
			Type:  paramType,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return parameters, nil
}
