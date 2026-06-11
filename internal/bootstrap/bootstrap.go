// Package bootstrap is the application's composition root: it constructs the
// concrete adapters (awsx, envfile) and wires them to the secrets ports,
// returning a ready-to-use *secrets.Service.
//
// It is the only place that imports both the policy layer (secrets) and the
// infrastructure adapters (awsx) — keeping awsx free of any app dependency and
// secrets free of any concrete SDK constructor.
package bootstrap

import (
	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/awsx/service"
	"hexyn-aws/internal/config"
	"hexyn-aws/internal/envfile"
	"hexyn-aws/internal/secrets"
)

// NewService assembles a Service from the given config provider.
func NewService(cfg *config.Provider) *secrets.Service {
	creds := awsx.NewCredentialStore(cfg)
	return secrets.New(secrets.Deps{
		Cfg:     cfg,
		Creds:   creds,
		Session: awsx.NewSessions(creds),
		Env:     envfile.FS{},
		AWS:     service.NewClients(creds),
	})
}
