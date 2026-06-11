package envfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hexyn-aws/internal/awsx"
)

// Write sorts params by name and writes them as a KEY=VALUE .env file at dir/fileName.
// SecureString parameters are annotated with a trailing " //secureString".
func (FS) Write(dir, fileName string, params []awsx.Parameter) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	sorted := make([]awsx.Parameter, len(params))
	copy(sorted, params)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	var sb strings.Builder
	for _, p := range sorted {
		sb.WriteString(p.Name)
		sb.WriteString("=")
		sb.WriteString(p.Value)
		if p.IsSecure() {
			sb.WriteString(" //secureString")
		}
		sb.WriteString("\n")
	}
	return os.WriteFile(filepath.Join(dir, fileName), []byte(sb.String()), 0o644)
}
