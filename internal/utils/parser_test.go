package utils

import (
	"os"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	content := `
# Comment line
DB_HOST=localhost
DB_PASSWORD=secret123 //secureString
API_KEY=key_val //SECURESTRING
NORMAL_VAR=hello
# Invalid format
INVALID_LINE
EMPTY=
=NO_KEY
   
// This is a comment at the end
`
	tmpFile, _ := os.CreateTemp("", "test.env")
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	params, err := ParseEnvFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ParseEnvFile failed: %v", err)
	}

	expected := []struct {
		name, val, pType string
	}{
		{"DB_HOST", "localhost", "String"},
		{"DB_PASSWORD", "secret123", "SecureString"},
		{"API_KEY", "key_val", "SecureString"},
		{"NORMAL_VAR", "hello", "String"},
		{"EMPTY", "", "String"},
	}

	if len(params) != len(expected) {
		t.Errorf("expected %d params, got %d", len(expected), len(params))
	}

	for i, exp := range expected {
		if params[i].Name != exp.name || params[i].Value != exp.val || params[i].Type != exp.pType {
			t.Errorf("item %d: expected %+v, got %+v", i, exp, params[i])
		}
	}
}

func TestParseEnvFileNotFound(t *testing.T) {
	_, err := ParseEnvFile("non_existent_file.env")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}
