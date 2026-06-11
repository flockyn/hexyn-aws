package envfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteRaw writes arbitrary bytes to dir/fileName, creating the directory if needed.
// Used for the raw task-definition secrets dump (tdf-secrets.json).
func (FS) WriteRaw(dir, fileName string, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, fileName), data, 0o644)
}
