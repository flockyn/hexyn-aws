package envfile

import (
	"bufio"
	"fmt"
	"os"

	"hexyn-aws/internal/awsx"
)

// Parse reads the given .env file into a slice of parameters.
func (f FS) Parse(filePath string) ([]awsx.Parameter, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var params []awsx.Parameter
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if p, ok := f.parseLine(scanner.Text()); ok {
			params = append(params, p)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	return params, nil
}
