package envfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteRaw(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`[{"name":"X","valueFrom":"/a/b"}]`)

	require.NoError(t, (FS{}).WriteRaw(dir, "tdf-secrets.json", raw))

	got, err := os.ReadFile(filepath.Join(dir, "tdf-secrets.json"))
	require.NoError(t, err)
	assert.Equal(t, string(raw), string(got))
}
