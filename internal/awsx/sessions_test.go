package awsx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"hexyn-aws/internal/awsx"
	mocks "hexyn-aws/test/mocks/awsx"
)

// Check short-circuits on a credential load error before touching AWS, returning
// the error and carrying through the credential source for the UI.
func TestCheckPropagatesCredentialLoadError(t *testing.T) {
	loader := mocks.NewMockCredentialLoader(t)
	loader.EXPECT().Load().Return(awsx.LoadedCredentials{Source: "env"}, errors.New("no creds"))

	sess, err := awsx.NewSessions(loader).Check(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "env", sess.CredSource)
}

// Check reports ErrCredentialsExpired when the access key/secret are absent,
// again without making any AWS call.
func TestCheckExpiredWhenKeysMissing(t *testing.T) {
	loader := mocks.NewMockCredentialLoader(t)
	loader.EXPECT().Load().Return(awsx.LoadedCredentials{
		Source:   "file",
		Profile:  "admin",
		CredsMap: map[string]string{}, // no aws_access_key_id / aws_secret_access_key
	}, nil)

	sess, err := awsx.NewSessions(loader).Check(context.Background())
	assert.ErrorIs(t, err, awsx.ErrCredentialsExpired)
	assert.Equal(t, "admin", sess.Profile)
}

// ListEnabledRegions surfaces a credential-load failure from BuildConfig rather
// than falling back to the region list.
func TestListEnabledRegionsPropagatesConfigError(t *testing.T) {
	loader := mocks.NewMockCredentialLoader(t)
	loader.EXPECT().Load().Return(awsx.LoadedCredentials{}, errors.New("boom"))

	_, err := awsx.NewSessions(loader).ListEnabledRegions(context.Background())
	assert.Error(t, err)
}
