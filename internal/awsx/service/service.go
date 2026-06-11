package service

import (
	"context"

	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/secrets"
)

// Clients adapts the awsx service constructors to the secrets.AWS port, building
// a region-scoped client on demand (the region is a runtime choice).
type Clients struct {
	creds awsx.CredentialLoader
}

// NewClients creates a Clients adapter backed by the given credential loader.
func NewClients(creds awsx.CredentialLoader) *Clients {
	return &Clients{creds: creds}
}

func (c *Clients) SSM(ctx context.Context, region string) (secrets.SSMClient, error) {
	return NewSSM(ctx, c.creds, region)
}

func (c *Clients) ECS(ctx context.Context, region string) (secrets.ECSClient, error) {
	return NewECS(ctx, c.creds, region)
}
