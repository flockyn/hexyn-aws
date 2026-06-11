// Package config resolves the on-disk locations the tool reads and writes:
// the base directory, the credentials file, and the input/output subdirectories.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	baseDirName     = ".hexyn-aws"
	credentialsFile = "credentials"
	inputSubDir     = "input"
	outputSubDir    = "output"
	gitignoreFile   = ".gitignore"
	gitignoreBody   = "# Ignore everything in this directory\n*\n!.gitignore\n"
)

// Provider holds the resolved base directory and init-mode flag. It is constructed
// once during wiring and injected wherever a path is needed.
type Provider struct {
	baseDir    string
	isInitMode bool
}

// New resolves the base directory. When local is true the current directory's
// .hexyn-aws folder is used; otherwise a local .hexyn-aws is preferred when present,
// falling back to the home-directory folder.
func New(local bool) *Provider {
	p := &Provider{isInitMode: local}
	if local {
		p.baseDir = baseDirName
		return p
	}
	if _, err := os.Stat(baseDirName); err == nil {
		p.baseDir = baseDirName
	} else {
		home, _ := os.UserHomeDir()
		p.baseDir = filepath.Join(home, baseDirName)
	}
	return p
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
	return nil
}
