// Package secrets orchestrates the tool's use cases (pull/push SSM parameters,
// session checks, region/cluster listing) over a set of small, injected ports
// (declared in ports.go). It is the application's policy layer and depends only
// on those interfaces, never on the AWS SDK directly.
package secrets

import (
	"context"
	"path/filepath"
	"strings"

	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/config"
)

// ParamTarget addresses an SSM path in a region (PullByPath / Push).
type ParamTarget struct {
	Env    string
	Repo   string
	Region string
}

// TaskTarget addresses an ECS service in a region (PullByTaskDef) and names the
// output subdirectory the resolved .env files are written into.
type TaskTarget struct {
	Region    string
	Cluster   string
	Service   string
	OutputDir string
}

// Deps bundles the Service's collaborators so New stays a single-argument constructor.
type Deps struct {
	Cfg     *config.Provider
	Creds   CredentialStore
	Session SessionClient
	Env     EnvFiles
	AWS     AWS
}

// Service is the application's use-case orchestrator.
type Service struct {
	cfg     *config.Provider
	creds   CredentialStore
	session SessionClient
	env     EnvFiles
	aws     AWS
}

// New constructs a Service from its dependencies.
func New(d Deps) *Service {
	return &Service{cfg: d.Cfg, creds: d.Creds, session: d.Session, env: d.Env, aws: d.AWS}
}

// CheckSession validates the current AWS session.
func (s *Service) CheckSession(ctx context.Context) (awsx.Session, error) {
	return s.session.Check(ctx)
}

// ListRegions returns the regions enabled for the account.
func (s *Service) ListRegions(ctx context.Context) ([]string, error) {
	return s.session.ListEnabledRegions(ctx)
}

// ListClusters returns the ECS clusters in the region.
func (s *Service) ListClusters(ctx context.Context, region string) ([]awsx.Cluster, error) {
	ecsClient, err := s.aws.ECS(ctx, region)
	if err != nil {
		return nil, err
	}
	return ecsClient.ListClusters(ctx)
}

// ListServices returns the ECS services in the cluster.
func (s *Service) ListServices(ctx context.Context, region, cluster string) ([]awsx.Service, error) {
	ecsClient, err := s.aws.ECS(ctx, region)
	if err != nil {
		return nil, err
	}
	return ecsClient.ListServices(ctx, cluster)
}

// PullByPath fetches all parameters under /<Env>/<Repo>/ and writes them to
// <OutputDir>/<subDir>/ps.env.
func (s *Service) PullByPath(ctx context.Context, t ParamTarget, subDir string) error {
	ssmClient, err := s.aws.SSM(ctx, t.Region)
	if err != nil {
		return err
	}
	params, err := ssmClient.GetByPath(ctx, awsx.ParamPath{Env: t.Env, Repo: t.Repo})
	if err != nil {
		return err
	}
	return s.env.Write(s.outputPath(subDir), "ps.env", params)
}

// PullByTaskDef resolves an ECS service's task-definition secrets, writes the raw
// mapping to tdf-secrets.json, resolves the SSM values (filling unresolved vars
// with empty values so the .env is complete), then writes them grouped by their
// SSM path prefix into separate <prefix>.env files — all under <OutputDir>/<sub>/.
func (s *Service) PullByTaskDef(ctx context.Context, t TaskTarget) error {
	ecsClient, err := s.aws.ECS(ctx, t.Region)
	if err != nil {
		return err
	}
	secrets, rawJSON, err := ecsClient.GetTaskSecrets(ctx, t.Cluster, t.Service)
	if err != nil {
		return err
	}

	dir := s.outputPath(t.OutputDir)
	if err := s.env.WriteRaw(dir, "tdf-secrets.json", rawJSON); err != nil {
		return err
	}

	nameMap := make(map[string]string, len(secrets))
	for _, sec := range secrets {
		nameMap[sec.EnvVarName] = sec.SSMPath
	}

	ssmClient, err := s.aws.SSM(ctx, t.Region)
	if err != nil {
		return err
	}
	params, err := ssmClient.GetByNames(ctx, nameMap)
	if err != nil {
		return err
	}
	params = s.fillMissing(params, nameMap)

	for group, gParams := range s.groupByPrefix(params, nameMap, t.OutputDir) {
		if err := s.env.Write(dir, group+".env", gParams); err != nil {
			return err
		}
	}
	return nil
}

// Push parses an input .env file and uploads its parameters to /<Env>/<Repo>/.
func (s *Service) Push(ctx context.Context, t ParamTarget, fileName string) (int, []error) {
	params, err := s.env.Parse(filepath.Join(s.cfg.InputDir(), fileName))
	if err != nil {
		return 0, []error{err}
	}
	ssmClient, err := s.aws.SSM(ctx, t.Region)
	if err != nil {
		return 0, []error{err}
	}
	return ssmClient.Put(ctx, awsx.ParamPath{Env: t.Env, Repo: t.Repo}, params)
}

// SaveCredentials persists temporary credentials derived from the given ARN.
func (s *Service) SaveCredentials(creds awsx.Credentials, arn string) error {
	return s.creds.Save(creds, arn)
}

// UpdateRegion rewrites the region in the credentials file.
func (s *Service) UpdateRegion(region string) error {
	return s.creds.UpdateRegion(region)
}

// outputPath returns the per-run output directory <OutputDir>/<subDir>.
func (s *Service) outputPath(subDir string) string {
	return filepath.Join(s.cfg.OutputDir(), subDir)
}

// groupByPrefix buckets resolved parameters into output files keyed by the group
// segment of their SSM path (e.g. /env/global/KEY → "global"), so task-definition
// secrets are split into per-prefix .env files. Parameters whose path yields no
// group fall back to the given file name. The receiver is unused; it keeps the
// helper grouped on Service.
func (*Service) groupByPrefix(params []awsx.Parameter, nameMap map[string]string, fallback string) map[string][]awsx.Parameter {
	groups := make(map[string][]awsx.Parameter)
	for _, p := range params {
		group := fallback
		if parts := strings.Split(nameMap[p.Name], "/"); len(parts) > 1 {
			group = parts[len(parts)-2]
		}
		groups[group] = append(groups[group], p)
	}
	return groups
}

// fillMissing appends empty-valued parameters for any env var that SSM did not
// resolve, ensuring the written .env contains every requested key. The receiver
// is unused; it keeps the helper grouped on Service.
func (*Service) fillMissing(params []awsx.Parameter, nameMap map[string]string) []awsx.Parameter {
	resolved := make(map[string]bool, len(params))
	for _, p := range params {
		resolved[p.Name] = true
	}
	for envVar := range nameMap {
		if !resolved[envVar] {
			params = append(params, awsx.Parameter{Name: envVar, Value: "", Type: awsx.ParameterTypeString})
		}
	}
	return params
}
