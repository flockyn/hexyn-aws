package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hexyn-aws/test/fixtures"
)

func TestNewInitModeUsesLocalDir(t *testing.T) {
	p := New(true)

	assert.True(t, p.IsInitMode(), "expected IsInitMode to be true")
	assert.Equal(t, ".hexyn-aws", p.BaseDir())
}

func TestNewPrefersLocalDirWhenPresent(t *testing.T) {
	dir := t.TempDir()
	fixtures.Chdir(t, dir)
	require.NoError(t, os.Mkdir(".hexyn-aws", 0o755))

	p := New(false)

	assert.Equal(t, ".hexyn-aws", p.BaseDir(), "expected local baseDir")
	assert.False(t, p.IsInitMode(), "expected IsInitMode false when local flag not set")
}

func TestNewFallsBackToHomeDir(t *testing.T) {
	fixtures.Chdir(t, t.TempDir()) // no local .hexyn-aws here

	p := New(false)

	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".hexyn-aws"), p.BaseDir(), "expected home baseDir")
}

func TestPathHelpers(t *testing.T) {
	p := New(true) // baseDir == ".hexyn-aws"

	assert.Equal(t, filepath.Join(".hexyn-aws", "credentials"), p.CredentialsPath())
	assert.Equal(t, filepath.Join(".hexyn-aws", "input"), p.InputDir())
	assert.Equal(t, filepath.Join(".hexyn-aws", "output"), p.OutputDir())
}

func TestEnsureDirectoriesCreatesTreeAndGitignore(t *testing.T) {
	fixtures.Chdir(t, t.TempDir())
	p := New(true)

	require.NoError(t, p.EnsureDirectories())

	for _, dir := range []string{p.InputDir(), p.OutputDir()} {
		info, err := os.Stat(dir)
		require.NoErrorf(t, err, "expected directory %q to exist", dir)
		assert.Truef(t, info.IsDir(), "expected %q to be a directory", dir)
	}
	gi := filepath.Join(p.BaseDir(), ".gitignore")
	_, err := os.Stat(gi)
	assert.NoErrorf(t, err, "expected .gitignore at %q", gi)
}
