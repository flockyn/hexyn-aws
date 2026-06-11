package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewInitModeUsesLocalDir(t *testing.T) {
	p := New(true)

	if !p.IsInitMode() {
		t.Fatal("expected IsInitMode to be true")
	}
	if p.BaseDir() != ".hexyn-aws" {
		t.Fatalf("expected baseDir %q, got %q", ".hexyn-aws", p.BaseDir())
	}
}

func TestNewPrefersLocalDirWhenPresent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.Mkdir(".hexyn-aws", 0o755); err != nil {
		t.Fatal(err)
	}

	p := New(false)

	if p.BaseDir() != ".hexyn-aws" {
		t.Fatalf("expected local baseDir, got %q", p.BaseDir())
	}
	if p.IsInitMode() {
		t.Fatal("expected IsInitMode false when local flag not set")
	}
}

func TestNewFallsBackToHomeDir(t *testing.T) {
	chdir(t, t.TempDir()) // no local .hexyn-aws here

	p := New(false)

	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".hexyn-aws")
	if p.BaseDir() != want {
		t.Fatalf("expected home baseDir %q, got %q", want, p.BaseDir())
	}
}

func TestPathHelpers(t *testing.T) {
	p := New(true) // baseDir == ".hexyn-aws"

	cases := map[string]string{
		p.CredentialsPath(): filepath.Join(".hexyn-aws", "credentials"),
		p.InputDir():        filepath.Join(".hexyn-aws", "input"),
		p.OutputDir():       filepath.Join(".hexyn-aws", "output"),
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("path mismatch: got %q want %q", got, want)
		}
	}
}

func TestEnsureDirectoriesCreatesTreeAndGitignore(t *testing.T) {
	chdir(t, t.TempDir())
	p := New(true)

	if err := p.EnsureDirectories(); err != nil {
		t.Fatalf("EnsureDirectories: %v", err)
	}

	for _, dir := range []string{p.InputDir(), p.OutputDir()} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Errorf("expected directory %q to exist", dir)
		}
	}
	gi := filepath.Join(p.BaseDir(), ".gitignore")
	if _, err := os.Stat(gi); err != nil {
		t.Errorf("expected .gitignore at %q: %v", gi, err)
	}
}

// chdir switches to dir for the duration of the test and restores the original cwd.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}
