package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRaw(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`[{"name":"X","valueFrom":"/a/b"}]`)

	if err := (FS{}).WriteRaw(dir, "tdf-secrets.json", raw); err != nil {
		t.Fatalf("WriteRaw: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "tdf-secrets.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Errorf("raw mismatch: got %q want %q", got, raw)
	}
}
