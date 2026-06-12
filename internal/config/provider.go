// Package config resolves the on-disk locations the tool reads and writes:
// the base directory, the credentials file, and the input/output subdirectories.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	baseDirName     = ".hexyn-aws"
	credentialsFile = "credentials"
	inputSubDir     = "input"
	outputSubDir    = "output"
	configFile      = "config"
	gitignoreFile   = ".gitignore"
	gitignoreBody   = "# Ignore everything in this directory\n*\n!.gitignore\n"

	// configBody is the commented example written to .hexyn-aws/config on first run.
	// Every line is a comment, so the built-in defaults stay in effect until edited.
	configBody = "# Hexyn AWS configuration (INI style).\n" +
		"#\n" +
		"# [repository]\n" +
		"# prefix: comma-separated service-name prefixes stripped to derive the SSM repo\n" +
		"# name (most specific first). Overridden by HEXYN_REPO_PREFIXES. Unset by default.\n" +
		"#\n" +
		"# [repository]\n" +
		"# prefix=service-,svc-\n"

	repoPrefixEnv     = "HEXYN_REPO_PREFIXES" // overrides the config-file value
	repoSection       = "repository"          // INI section holding repo settings
	repoPrefixKey     = "prefix"              // INI key under [repository]
	repoPrefixSetting = repoSection + "." + repoPrefixKey
)

// defaultRepoPrefixes is the fallback when neither the config file nor the
// environment provides prefixes: empty, so nothing is stripped unless configured.
var defaultRepoPrefixes []string

// Provider holds the resolved base directory, init-mode flag, and the parsed
// config-file settings. It is constructed once during wiring and injected wherever
// a path is needed.
type Provider struct {
	baseDir    string
	isInitMode bool
	settings   map[string]string // INI "section.key" → value (lower-cased keys)
}

// New resolves the base directory and loads the config file. When local is true the
// current directory's .hexyn-aws folder is used; otherwise a local .hexyn-aws is
// preferred when present, falling back to the home-directory folder.
func New(local bool) *Provider {
	p := &Provider{isInitMode: local}
	if local {
		p.baseDir = baseDirName
	} else if _, err := os.Stat(baseDirName); err == nil {
		p.baseDir = baseDirName
	} else {
		home, _ := os.UserHomeDir()
		p.baseDir = filepath.Join(home, baseDirName)
	}
	p.settings = p.parseConfigFile(filepath.Join(p.baseDir, configFile))
	return p
}

// parseConfigFile reads an INI-style settings file ("[section]" headers and
// "key=value" pairs). A missing or unreadable file yields an empty map; blank lines
// and "#"/";" comments are skipped. Keys are stored as lower-cased "section.key" so
// lookups are case-insensitive. The receiver is unused; it keeps the helper grouped
// on Provider.
func (*Provider) parseConfigFile(path string) map[string]string {
	settings := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return settings
	}
	section := ""
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if section != "" {
			key = section + "." + key
		}
		settings[key] = strings.TrimSpace(value)
	}
	return settings
}

// BaseDir returns the resolved base directory path.
func (p *Provider) BaseDir() string { return p.baseDir }

// IsInitMode reports whether the tool was started with the --init flag.
func (p *Provider) IsInitMode() bool { return p.isInitMode }

// CredentialsPath returns the absolute path to the credentials file.
func (p *Provider) CredentialsPath() string {
	return filepath.Join(p.baseDir, credentialsFile)
}

// InputDir returns the path where .env input files should be placed.
func (p *Provider) InputDir() string {
	return filepath.Join(p.baseDir, inputSubDir)
}

// OutputDir returns the path where retrieved .env files are written.
func (p *Provider) OutputDir() string {
	return filepath.Join(p.baseDir, outputSubDir)
}

// RepoNamePrefixes returns the ordered list of service-name prefixes stripped to
// derive a repo name. Precedence: the HEXYN_REPO_PREFIXES env var, then the
// [repository] prefix config-file value (both comma-separated, e.g.
// "team-service-,team-"), then a generic default. List more specific prefixes first.
func (p *Provider) RepoNamePrefixes() []string {
	if env := p.splitTrim(os.Getenv(repoPrefixEnv), ","); len(env) > 0 {
		return env
	}
	if fromFile := p.splitTrim(p.settings[repoPrefixSetting], ","); len(fromFile) > 0 {
		return fromFile
	}
	return defaultRepoPrefixes
}

// ConfigPath returns the path to the settings file.
func (p *Provider) ConfigPath() string {
	return filepath.Join(p.baseDir, configFile)
}

// SetRepoPrefixes updates the repo-name prefixes in memory and persists the config
// file. An empty list clears the setting (falling back to env/default on next read).
func (p *Provider) SetRepoPrefixes(prefixes []string) error {
	if p.settings == nil {
		p.settings = make(map[string]string)
	}
	if len(prefixes) == 0 {
		delete(p.settings, repoPrefixSetting)
	} else {
		p.settings[repoPrefixSetting] = strings.Join(prefixes, ",")
	}
	return p.save()
}

// save writes the current settings back to the config file in INI style.
func (p *Provider) save() error {
	if p.baseDir == "" {
		return fmt.Errorf("base directory not configured")
	}
	if err := os.MkdirAll(p.baseDir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Hexyn AWS configuration (managed via the app; INI style).\n")
	if pfx := p.settings[repoPrefixSetting]; pfx != "" {
		b.WriteString("\n[")
		b.WriteString(repoSection)
		b.WriteString("]\n")
		b.WriteString(repoPrefixKey)
		b.WriteString("=")
		b.WriteString(pfx)
		b.WriteString("\n")
	}
	return os.WriteFile(filepath.Join(p.baseDir, configFile), []byte(b.String()), 0o644)
}

// splitTrim splits raw on sep, trimming each element and dropping the empties. The
// receiver is unused; it keeps the helper grouped on Provider.
func (*Provider) splitTrim(raw, sep string) []string {
	var out []string
	for part := range strings.SplitSeq(raw, sep) {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// EnsureDirectories creates the input/output subdirectories and a protective
// .gitignore if they do not yet exist.
func (p *Provider) EnsureDirectories() error {
	if p.baseDir == "" {
		return fmt.Errorf("base directory not configured")
	}
	if err := os.MkdirAll(p.InputDir(), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(p.OutputDir(), 0o755); err != nil {
		return err
	}
	gi := filepath.Join(p.baseDir, gitignoreFile)
	if _, err := os.Stat(gi); os.IsNotExist(err) {
		_ = os.WriteFile(gi, []byte(gitignoreBody), 0o644)
	}
	cfg := filepath.Join(p.baseDir, configFile)
	if _, err := os.Stat(cfg); os.IsNotExist(err) {
		_ = os.WriteFile(cfg, []byte(configBody), 0o644)
	}
	return nil
}
