package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/config"
	"hexyn-aws/test/fixtures"
)

func TestClientsErrorWithoutCredentials(t *testing.T) {
	fixtures.Chdir(t, t.TempDir()) // no .hexyn-aws/credentials here

	c := NewClients(awsx.NewCredentialStore(config.New(true)))
	_, err := c.SSM(context.Background(), "us-east-1")
	assert.Error(t, err, "expected SSM to error when credentials are missing")
	_, err = c.ECS(context.Background(), "us-east-1")
	assert.Error(t, err, "expected ECS to error when credentials are missing")
}
