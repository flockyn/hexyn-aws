package envfile

import (
	"os"
	"path/filepath"
	"testing"

	"hexyn-aws/internal/awsx"
)

func TestWriteSortsAndAnnotates(t *testing.T) {
	dir := t.TempDir()
	params := []awsx.Parameter{
		{Name: "ZED", Value: "last"},
		{Name: "API_KEY", Value: "secret", Type: awsx.ParameterTypeSecureString},
		{Name: "ALPHA", Value: "first"},
	}

	if err := (FS{}).Write(dir, "out.env", params); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "out.env"))
	if err != nil {
		t.Fatal(err)
	}
	want := "ALPHA=first\nAPI_KEY=secret //secureString\nZED=last\n"
	if string(data) != want {
		t.Errorf("unexpected output:\n got: %q\nwant: %q", data, want)
	}
}

// TestWriteParseRoundTripURL verifies Write produces output that Parse reads back
// losslessly — including a secure URL whose value contains "//".
func TestWriteParseRoundTripURL(t *testing.T) {
	dir := t.TempDir()
	in := []awsx.Parameter{
		{Name: "RABBITMQ_URL", Value: "amqp://guest:guest@rabbitmq-5672-tcp:5672/", Type: awsx.ParameterTypeSecureString},
		{Name: "PLAIN_URL", Value: "http://example.com//x", Type: awsx.ParameterTypeString},
	}
	if err := (FS{}).Write(dir, "rt.env", in); err != nil {
		t.Fatal(err)
	}
	out, err := FS{}.Parse(filepath.Join(dir, "rt.env"))
	if err != nil {
		t.Fatal(err)
	}

	byName := map[string]awsx.Parameter{}
	for _, p := range out {
		byName[p.Name] = p
	}
	if got := byName["RABBITMQ_URL"]; got.Value != in[0].Value || !got.IsSecure() {
		t.Errorf("secure URL round-trip failed: %+v", got)
	}
	if got := byName["PLAIN_URL"]; got.Value != in[1].Value || got.IsSecure() {
		t.Errorf("plain URL round-trip failed: %+v", got)
	}
}
