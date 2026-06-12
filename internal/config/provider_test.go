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

func TestRepoNamePrefixesEmptyWhenUnset(t *testing.T) {
	fixtures.Chdir(t, t.TempDir()) // no config file, no env
	t.Setenv(repoPrefixEnv, "")
	assert.Empty(t, New(true).RepoNamePrefixes(), "no prefixes are stripped unless configured")
}

func TestRepoNamePrefixesReadsEnvAndTrims(t *testing.T) {
	fixtures.Chdir(t, t.TempDir())
	t.Setenv(repoPrefixEnv, " team-service- , team- ,, service-")
	assert.Equal(t, []string{"team-service-", "team-", "service-"}, New(true).RepoNamePrefixes(),
		"prefixes should be split, trimmed, and blanks dropped, preserving order")
}

func TestRepoNamePrefixesReadsConfigFile(t *testing.T) {
	fixtures.Chdir(t, t.TempDir())
	t.Setenv(repoPrefixEnv, "")
	writeConfig(t, "# repo prefixes\n[repository]\nprefix=team-service-,team-\n")

	assert.Equal(t, []string{"team-service-", "team-"}, New(true).RepoNamePrefixes(),
		"comma-separated [repository] prefix values should be read in order")
}

func TestRepoNamePrefixesEnvOverridesConfigFile(t *testing.T) {
	fixtures.Chdir(t, t.TempDir())
	writeConfig(t, "[repository]\nprefix=from-file-\n")
	t.Setenv(repoPrefixEnv, "from-env-")

	assert.Equal(t, []string{"from-env-"}, New(true).RepoNamePrefixes(),
		"the env var should take precedence over the config file")
}

// writeConfig creates .hexyn-aws/config in the current directory with the given body.
func writeConfig(t *testing.T, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(baseDirName, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDirName, configFile), []byte(body), 0o644))
}

func TestSetRepoPrefixesPersistsAndReloads(t *testing.T) {
	fixtures.Chdir(t, t.TempDir())
	t.Setenv(repoPrefixEnv, "") // env must not mask the config file

	p := New(true)
	require.NoError(t, p.SetRepoPrefixes([]string{"team-service-", "team-"}))

	// Visible immediately on the same provider...
	assert.Equal(t, []string{"team-service-", "team-"}, p.RepoNamePrefixes())
	// ...and persisted, so a freshly constructed provider reads it back.
	assert.Equal(t, []string{"team-service-", "team-"}, New(true).RepoNamePrefixes())
}

func TestSetRepoPrefixesEmptyClearsToDefault(t *testing.T) {
	fixtures.Chdir(t, t.TempDir())
	t.Setenv(repoPrefixEnv, "")

	p := New(true)
	require.NoError(t, p.SetRepoPrefixes([]string{"team-"}))
	require.NoError(t, p.SetRepoPrefixes(nil))

	assert.Empty(t, New(true).RepoNamePrefixes(), "cleared config strips nothing")
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

	cfg := filepath.Join(p.BaseDir(), configFile)
	_, err = os.Stat(cfg)
	assert.NoErrorf(t, err, "expected example config at %q", cfg)
}
