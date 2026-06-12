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
	// Cluster and Service identify the ECS service whose task definition a path
	// export is diffed against (to surface repo env vars the TDF expects but the
	// path did not return). They are unused by Push.
	Cluster string
	Service string
}

// TaskTarget addresses an ECS service in a region (PullByTaskDef) and names the
// output subdirectory the resolved .env files are written into. Env and Repo name
// the /<Env>/<Repo>/ prefix the export is reconciled against (Repo is derived from
// the service name).
type TaskTarget struct {
	Region    string
	Cluster   string
	Service   string
	OutputDir string
	Env       string
	Repo      string
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

// PullByPath fetches all parameters under /<Env>/<Repo>/, splits them into the
// ones the task definition actually sources from this path (used) and the rest
// (unused), and writes them under <OutputDir>/<subDir>/. Used params are grouped
// by their sub-path segment into <group>.env files (root-level params go to
// <Repo>.env). It always writes the per-repo trio even when empty: <Repo>.env,
// <Repo>.unused.env (in SSM here but not sourced from this path by the TDF) and
// <Repo>.missing.env (sourced from this path by the TDF but absent from SSM).
func (s *Service) PullByPath(ctx context.Context, t ParamTarget, subDir string) error {
	ssmClient, err := s.aws.SSM(ctx, t.Region)
	if err != nil {
		return err
	}
	params, err := ssmClient.GetByPath(ctx, awsx.ParamPath{Env: t.Env, Repo: t.Repo})
	if err != nil {
		return err
	}

	ecsClient, err := s.aws.ECS(ctx, t.Region)
	if err != nil {
		return err
	}
	taskSecrets, _, err := ecsClient.GetTaskSecrets(ctx, t.Cluster, t.Service)
	if err != nil {
		return err
	}

	used, unused, missing := s.compareWithTaskDef("/"+t.Env+"/"+t.Repo+"/", params, taskSecrets)

	// Only the used params populate the grouped files; shadows (in SSM but sourced
	// elsewhere by the TDF) and orphans land in <repo>.unused.env instead.
	files := make(map[string][]awsx.Parameter)
	for group, gParams := range s.groupByPathSegment(used, t.Repo) {
		files[group+".env"] = gParams
	}
	// Guarantee the per-repo trio exists even when empty: <repo>.env holds the
	// root-level params (left empty if there are none); the two reconciliation
	// files are always written so each export yields a predictable, complete set.
	if _, ok := files[t.Repo+".env"]; !ok {
		files[t.Repo+".env"] = nil
	}
	files[t.Repo+".unused.env"] = unused
	files[t.Repo+".missing.env"] = missing
	return s.writeAll(s.outputPath(subDir), files)
}

// compareWithTaskDef reconciles the parameters found under prefix against a
// service's task-definition secrets, returning three sets:
//   - used: params the TDF sources directly from this prefix (valueFrom points at
//     the param's exact path). These are the ones the service actually reads here.
//   - unused: params present in SSM under the prefix that the TDF does not source
//     from this path — orphans, and "shadows" whose env var the service sources
//     from a different/shared prefix. A shadow whose key matches a TDF secret's
//     source key is relabeled with that secret's env-var name (e.g. a repo
//     "CACHE_URL" becomes "CACHE_HOST" when the TDF maps CACHE_HOST ← .../CACHE_URL),
//     keeping its value.
//   - missing: env vars the TDF sources from under this prefix but that are absent
//     from Parameter Store (empty-valued). Secrets sourced from a different repo or
//     a shared/global prefix are out of scope and skipped.
//
// The receiver is used only for the lastSegment helper; it keeps the comparison
// grouped on Service.
func (s *Service) compareWithTaskDef(prefix string, pathParams []awsx.Parameter, taskSecrets []awsx.TaskSecret) (used, unused, missing []awsx.Parameter) {
	tdfPaths := make(map[string]bool, len(taskSecrets))
	sourceKeyToName := make(map[string]string, len(taskSecrets))
	for _, sec := range taskSecrets {
		tdfPaths[sec.Path()] = true
		sourceKeyToName[s.lastSegment(sec.Path())] = sec.EnvVarName
	}

	for _, p := range pathParams {
		if tdfPaths[prefix+p.Name] { // the TDF sources this exact path → used here
			used = append(used, p)
			continue
		}
		entry := p
		if name, ok := sourceKeyToName[s.lastSegment(p.Name)]; ok {
			entry.Name = name // label by the env-var name the TDF exposes this key as
		}
		unused = append(unused, entry)
	}

	pathSet := make(map[string]bool, len(pathParams))
	for _, p := range pathParams {
		pathSet[prefix+p.Name] = true
	}
	seen := make(map[string]bool)
	for _, sec := range taskSecrets {
		ssmPath := sec.Path()
		if !strings.HasPrefix(ssmPath, prefix) { // other repo / shared / global → ignore
			continue
		}
		if pathSet[ssmPath] || seen[sec.EnvVarName] {
			continue
		}
		seen[sec.EnvVarName] = true
		missing = append(missing, awsx.Parameter{Name: sec.EnvVarName, Value: "", Type: awsx.ParameterTypeString})
	}
	return used, unused, missing
}

// lastSegment returns the final "/"-separated component of an SSM path tail, which
// is the env-var key a path parameter maps to (e.g. "global/KEY" → "KEY"). The
// receiver is unused; it keeps the helper grouped on Service.
func (*Service) lastSegment(name string) string {
	if i := strings.LastIndex(name, "/"); i != -1 {
		return name[i+1:]
	}
	return name
}

// writeAll writes each named parameter set as a .env file. Empty sets produce
// empty files, so callers can guarantee a file is always present.
func (s *Service) writeAll(dir string, files map[string][]awsx.Parameter) error {
	for name, params := range files {
		if err := s.env.Write(dir, name, params); err != nil {
			return err
		}
	}
	return nil
}

// PullByTaskDef resolves an ECS service's task-definition secrets, writes the raw
// mapping to tdf-secrets.json, resolves the SSM values (filling unresolved vars
// with empty values so the .env is complete), then writes them grouped by their
// SSM path prefix into separate <prefix>.env files — all under <OutputDir>/<sub>/.
// It then reconciles the /<Env>/<Repo>/ path against the task definition, always
// writing the per-repo trio <Repo>.env, <Repo>.unused.env and <Repo>.missing.env
// — even when empty (see PullByPath / compareWithTaskDef).
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

	files := make(map[string][]awsx.Parameter)
	for group, gParams := range s.groupByPrefix(params, nameMap, t.OutputDir) {
		files[group+".env"] = gParams
	}

	pathParams, err := ssmClient.GetByPath(ctx, awsx.ParamPath{Env: t.Env, Repo: t.Repo})
	if err != nil {
		return err
	}
	// Same per-repo trio as the path export, always present (see PullByPath). The
	// grouped files here come from the resolved TDF secrets, so the "used" split is
	// not needed — only the unused/missing reconciliation against the repo path.
	if _, ok := files[t.Repo+".env"]; !ok {
		files[t.Repo+".env"] = nil
	}
	_, unused, missing := s.compareWithTaskDef("/"+t.Env+"/"+t.Repo+"/", pathParams, secrets)
	files[t.Repo+".unused.env"] = unused
	files[t.Repo+".missing.env"] = missing
	return s.writeAll(dir, files)
}

// PreviewPush parses the input .env file without uploading anything, so the
// caller can show the user exactly what Push would upload before confirming.
func (s *Service) PreviewPush(fileName string) ([]awsx.Parameter, error) {
	return s.env.Parse(filepath.Join(s.cfg.InputDir(), fileName))
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

// groupByPathSegment buckets path-prefixed parameters (whose Name is the SSM
// path tail, e.g. "global/KEY") into output files keyed by the segment directly
// above the key (so /env/repo/global/KEY → group "global", key "KEY"). Keys with
// no nested segment fall back to the given repo name. The receiver is unused; it
// keeps the helper grouped on Service.
func (*Service) groupByPathSegment(params []awsx.Parameter, fallback string) map[string][]awsx.Parameter {
	groups := make(map[string][]awsx.Parameter)
	for _, p := range params {
		group, name := fallback, p.Name
		if parts := strings.Split(p.Name, "/"); len(parts) > 1 {
			group = parts[len(parts)-2]
			name = parts[len(parts)-1]
		}
		groups[group] = append(groups[group], awsx.Parameter{Name: name, Value: p.Value, Type: p.Type})
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
