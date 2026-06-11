package awsx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hexyn-aws/internal/awsx"
	mocks "hexyn-aws/test/mocks/awsx"
)

func TestBuildConfigExplicitRegionWins(t *testing.T) {
	loader := mocks.NewMockCredentialLoader(t)
	loader.EXPECT().Load().Return(awsx.LoadedCredentials{CredsMap: map[string]string{"region": "ap-southeast-3"}}, nil)

	cfg, err := awsx.BuildConfig(context.Background(), loader, "us-east-1")
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", cfg.Region, "explicit region should win")
}

func TestBuildConfigFallsBackToCredsRegion(t *testing.T) {
	loader := mocks.NewMockCredentialLoader(t)
	loader.EXPECT().Load().Return(awsx.LoadedCredentials{CredsMap: map[string]string{"region": "ap-southeast-3"}}, nil)

	cfg, err := awsx.BuildConfig(context.Background(), loader, "")
	require.NoError(t, err)
	assert.Equal(t, "ap-southeast-3", cfg.Region, "expected fallback to creds region")
}

func TestBuildConfigPropagatesLoadError(t *testing.T) {
	loader := mocks.NewMockCredentialLoader(t)
	loader.EXPECT().Load().Return(awsx.LoadedCredentials{}, errors.New("no creds"))

	_, err := awsx.BuildConfig(context.Background(), loader, "us-east-1")
	assert.Error(t, err, "expected error when credential load fails")
}
