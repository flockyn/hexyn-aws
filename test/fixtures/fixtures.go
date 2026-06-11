// Package fixtures holds shared, package-agnostic test helpers and sample data
// used across the suite. It deliberately imports no project packages so any
// test (white-box included) can use it without an import cycle.
package fixtures

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// WriteTemp writes body to a temp .env file and returns its path.
func WriteTemp(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in.env")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// Chdir switches to dir for the duration of the test, restoring the original
// working directory afterwards.
func Chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// PEMPublicKey is a sample multi-line RSA public key used to exercise PEM
// handling in the env-file parser/writer.
const PEMPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3rOvOl+++G8umr5Ax4sf
+P5HM7qYSqqrxUv4ww42/FrdwGnxcFNpfmOD5bDtV2SwlG5rTUmLovOzi64MJ6y6
AYvbyVFSFTaXadgRa5sU3oMYwu+t0eUzngCa+T1oeSA+E7eymWPFcYuIUFg3kck6
Si7qy5tzfrzIXIyba8r8r+M923KgdmXF+6AxqO/nKkL8EUaDkSwOYcV/9Uk2BwDk
96VdLeU4A71lO7mMCZVoVPgFMZPpXPfiPfdRswXomCE7n6YgZ5T4bIUj1RhEbHDn
jRdk7dX3hdCvma72tNNpMB0Yl7m+wrGnstORSQYsSLRgS6m+mF8gztDvfbLZyS6k
JQIDAQAB
-----END PUBLIC KEY-----`
