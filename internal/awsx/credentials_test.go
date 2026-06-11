package awsx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hexyn-aws/internal/config"
	"hexyn-aws/test/fixtures"
)

// newStore returns a CredentialStore rooted in an isolated temp directory.
func newStore(t *testing.T) *CredentialStore {
	t.Helper()
	fixtures.Chdir(t, t.TempDir())
	return NewCredentialStore(config.New(true)) // baseDir == ".hexyn-aws"
}

func TestCredentialStoreMissing(t *testing.T) {
	s := newStore(t)
	_, err := s.Load()
	assert.ErrorIs(t, err, ErrCredentialsMissing)
}

func TestCredentialStoreSaveLoadRoundTrip(t *testing.T) {
	s := newStore(t)
	creds := Credentials{AccessKeyID: "AKIA", SecretAccessKey: "secret", SessionToken: "token"}

	require.NoError(t, s.Save(creds, "arn:aws:sts::123:assumed-role/admin/session"))

	loaded, err := s.Load()
	require.NoError(t, err)
	assert.Equal(t, "admin", loaded.Profile)
	assert.Equal(t, "AKIA", loaded.CredsMap["aws_access_key_id"], "access key not round-tripped")
}

func TestCredentialStoreUpdateRegion(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.Save(Credentials{AccessKeyID: "AKIA"}, "default"))

	require.NoError(t, s.UpdateRegion("ap-southeast-3"))
	loaded, _ := s.Load()
	assert.Equal(t, "ap-southeast-3", loaded.CredsMap["region"], "region not updated")

	// Updating again should replace, not duplicate.
	require.NoError(t, s.UpdateRegion("us-east-1"))
	loaded, _ = s.Load()
	assert.Equal(t, "us-east-1", loaded.CredsMap["region"], "region not replaced")
}
