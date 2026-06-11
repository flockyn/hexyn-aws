package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in.env")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseReadsAllPairs(t *testing.T) {
	path := writeTemp(t, "FOO=bar\nBAZ=qux\n")

	got, err := FS{}.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 params, got %d", len(got))
	}
	if got[0].Name != "FOO" || got[0].Value != "bar" {
		t.Errorf("unexpected first param: %+v", got[0])
	}
}

func TestParseSkipsBlankAndCommentLines(t *testing.T) {
	path := writeTemp(t, "\n# a comment\nFOO=bar\n   \n")

	got, err := FS{}.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 param, got %d (%+v)", len(got), got)
	}
}

func TestParseMissingFile(t *testing.T) {
	if _, err := (FS{}).Parse(filepath.Join(t.TempDir(), "nope.env")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
