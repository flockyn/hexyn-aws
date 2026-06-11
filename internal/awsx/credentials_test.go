package awsx

import (
	"os"
	"testing"

	"hexyn-aws/internal/config"
)

// newStore returns a CredentialStore rooted in an isolated temp directory.
func newStore(t *testing.T) *CredentialStore {
	t.Helper()
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return NewCredentialStore(config.New(true)) // baseDir == ".hexyn-aws"
}

func TestCredentialStoreMissing(t *testing.T) {
	s := newStore(t)
	if _, err := s.Load(); err != ErrCredentialsMissing {
		t.Fatalf("expected ErrCredentialsMissing, got %v", err)
	}
}

func TestCredentialStoreSaveLoadRoundTrip(t *testing.T) {
	s := newStore(t)
	creds := Credentials{AccessKeyID: "AKIA", SecretAccessKey: "secret", SessionToken: "token"}

	if err := s.Save(creds, "arn:aws:sts::123:assumed-role/admin/session"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Profile != "admin" {
		t.Errorf("expected profile 'admin', got %q", loaded.Profile)
	}
	if loaded.CredsMap["aws_access_key_id"] != "AKIA" {
		t.Errorf("access key not round-tripped: %q", loaded.CredsMap["aws_access_key_id"])
	}
}

func TestCredentialStoreUpdateRegion(t *testing.T) {
	s := newStore(t)
	if err := s.Save(Credentials{AccessKeyID: "AKIA"}, "default"); err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateRegion("ap-southeast-3"); err != nil {
		t.Fatalf("UpdateRegion: %v", err)
	}
	loaded, _ := s.Load()
	if loaded.CredsMap["region"] != "ap-southeast-3" {
		t.Errorf("region not updated: %q", loaded.CredsMap["region"])
	}

	// Updating again should replace, not duplicate.
	if err := s.UpdateRegion("us-east-1"); err != nil {
		t.Fatal(err)
	}
	loaded, _ = s.Load()
	if loaded.CredsMap["region"] != "us-east-1" {
		t.Errorf("region not replaced: %q", loaded.CredsMap["region"])
	}
}
