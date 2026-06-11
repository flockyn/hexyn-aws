package secrets

import (
	"context"

	"hexyn-aws/internal/awsx"
)

// This file declares the outbound ports the Service depends on. Concrete
// implementations live in the awsx, envfile, and cmd/cli packages and are
// injected via Deps — the Service never imports them directly.

// SSMClient is a region-scoped view of SSM Parameter Store.
type SSMClient interface {
	GetByPath(ctx context.Context, dest awsx.ParamPath) ([]awsx.Parameter, error)
	GetByNames(ctx context.Context, nameMap map[string]string) ([]awsx.Parameter, error)
	Put(ctx context.Context, dest awsx.ParamPath, params []awsx.Parameter) (int, []error)
}

// ECSClient is a region-scoped view of ECS.
type ECSClient interface {
	ListClusters(ctx context.Context) ([]awsx.Cluster, error)
	ListServices(ctx context.Context, cluster string) ([]awsx.Service, error)
	GetTaskSecrets(ctx context.Context, cluster, service string) ([]awsx.TaskSecret, []byte, error)
}

// AWS builds region-scoped clients on demand (the region is a runtime choice).
type AWS interface {
	SSM(ctx context.Context, region string) (SSMClient, error)
	ECS(ctx context.Context, region string) (ECSClient, error)
}

// SessionClient validates the active session and lists regions.
type SessionClient interface {
	Check(ctx context.Context) (awsx.Session, error)
	ListEnabledRegions(ctx context.Context) ([]string, error)
}

// CredentialStore mutates the on-disk credentials file.
type CredentialStore interface {
	Save(creds awsx.Credentials, arn string) error
	UpdateRegion(region string) error
}

// EnvFiles reads and writes .env / raw files on disk.
type EnvFiles interface {
	Parse(path string) ([]awsx.Parameter, error)
	Write(dir, name string, params []awsx.Parameter) error
	WriteRaw(dir, name string, data []byte) error
}
