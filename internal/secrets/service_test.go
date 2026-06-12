package secrets_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/config"
	"hexyn-aws/internal/secrets"
	mocks "hexyn-aws/test/mocks/secrets"
)

// writeCall records an EnvFiles.Write invocation for assertion.
type writeCall struct {
	dir, name string
	params    []awsx.Parameter
}

// deps groups the mocked ports; buildService fills any nil entry with an empty
// mock so a test only wires the dependencies it actually exercises.
type deps struct {
	aws   secrets.AWS
	env   secrets.EnvFiles
	creds secrets.CredentialStore
	sess  secrets.SessionClient
}

func buildService(t *testing.T, d deps) (*secrets.Service, *config.Provider) {
	t.Helper()
	if d.aws == nil {
		d.aws = mocks.NewMockAWS(t)
	}
	if d.env == nil {
		d.env = mocks.NewMockEnvFiles(t)
	}
	if d.creds == nil {
		d.creds = mocks.NewMockCredentialStore(t)
	}
	if d.sess == nil {
		d.sess = mocks.NewMockSessionClient(t)
	}
	cfg := config.New(true) // OutputDir == ".hexyn-aws/output"
	return secrets.New(secrets.Deps{Cfg: cfg, Creds: d.creds, Session: d.sess, Env: d.env, AWS: d.aws}), cfg
}

func TestPullByPathGroupsBySubPrefix(t *testing.T) {
	ssm := mocks.NewMockSSMClient(t)
	ssm.EXPECT().
		GetByPath(mock.Anything, awsx.ParamPath{Env: "prod", Repo: "api"}).
		Return([]awsx.Parameter{
			{Name: "DB_HOST", Value: "x"},         // no nested segment → falls back to <repo>.env
			{Name: "global/SHARED", Value: "tok"}, // nested → global.env, key trimmed to SHARED
		}, nil)

	// The TDF references exactly the two path params, so there is nothing unused
	// and nothing missing — only the grouped files are written.
	ecs := mocks.NewMockECSClient(t)
	ecs.EXPECT().GetTaskSecrets(mock.Anything, "prod", "api").Return([]awsx.TaskSecret{
		{EnvVarName: "DB_HOST", SSMPath: "arn:aws:ssm:ap-southeast-3:111:parameter/prod/api/DB_HOST"},
		{EnvVarName: "SHARED", SSMPath: "arn:aws:ssm:ap-southeast-3:111:parameter/prod/api/global/SHARED"},
	}, []byte("[]"), nil)

	aws := mocks.NewMockAWS(t)
	aws.EXPECT().SSM(mock.Anything, "ap-southeast-3").Return(ssm, nil)
	aws.EXPECT().ECS(mock.Anything, "ap-southeast-3").Return(ecs, nil)

	env := mocks.NewMockEnvFiles(t)
	var writes []writeCall
	env.EXPECT().Write(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(dir, name string, p []awsx.Parameter) error {
			writes = append(writes, writeCall{dir, name, p})
			return nil
		})

	svc, cfg := buildService(t, deps{aws: aws, env: env})
	require.NoError(t, svc.PullByPath(context.Background(), secrets.ParamTarget{
		Env: "prod", Repo: "api", Region: "ap-southeast-3", Cluster: "prod", Service: "api",
	}, "api"))

	wantDir := filepath.Join(cfg.OutputDir(), "api")
	byFile := map[string]map[string]string{}
	for _, w := range writes {
		assert.Equalf(t, wantDir, w.dir, "write %q in wrong dir", w.name)
		vals := map[string]string{}
		for _, p := range w.params {
			vals[p.Name] = p.Value
		}
		byFile[w.name] = vals
	}

	// Grouped files plus the always-present (here empty) reconciliation pair.
	require.Lenf(t, byFile, 4, "expected api.env, global.env, api.unused.env, api.missing.env, got %v", byFile)
	assert.Equal(t, "x", byFile["api.env"]["DB_HOST"], "root param should land in <repo>.env")
	assert.Equal(t, "tok", byFile["global.env"]["SHARED"], "nested param should be grouped and key trimmed")
	assert.Empty(t, byFile["api.unused.env"], "nothing unused when the TDF references every path param")
	assert.Empty(t, byFile["api.missing.env"], "nothing missing when the TDF is fully present on the path")
}

func TestPullByPathComparesWithTaskDef(t *testing.T) {
	ssm := mocks.NewMockSSMClient(t)
	ssm.EXPECT().GetByPath(mock.Anything, mock.Anything).
		Return([]awsx.Parameter{
			{Name: "DB_HOST", Value: "x"},  // referenced by TDF → in neither diff
			{Name: "LEGACY", Value: "old"}, // not referenced by TDF → unused
		}, nil)

	// TDF expects DB_HOST (present on path), API_KEY under the repo (absent on path
	// → missing), and a shared SHARED_TOKEN (excluded from a path export). The
	// valueFrom is a full ARN, as ECS reports it — the diff must normalize it.
	ecs := mocks.NewMockECSClient(t)
	ecs.EXPECT().GetTaskSecrets(mock.Anything, "prod", "api").Return([]awsx.TaskSecret{
		{EnvVarName: "DB_HOST", SSMPath: "arn:aws:ssm:ap-southeast-3:111:parameter/prod/api/DB_HOST"},
		{EnvVarName: "API_KEY", SSMPath: "arn:aws:ssm:ap-southeast-3:111:parameter/prod/api/api_key"},
		{EnvVarName: "SHARED_TOKEN", SSMPath: "arn:aws:ssm:ap-southeast-3:111:parameter/prod/global/token"},
	}, []byte("[]"), nil)

	aws := mocks.NewMockAWS(t)
	aws.EXPECT().SSM(mock.Anything, "r").Return(ssm, nil)
	aws.EXPECT().ECS(mock.Anything, "r").Return(ecs, nil)

	env := mocks.NewMockEnvFiles(t)
	var writes []writeCall
	env.EXPECT().Write(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(dir, name string, p []awsx.Parameter) error {
			writes = append(writes, writeCall{dir, name, p})
			return nil
		})

	svc, _ := buildService(t, deps{aws: aws, env: env})
	require.NoError(t, svc.PullByPath(context.Background(), secrets.ParamTarget{
		Env: "prod", Repo: "api", Region: "r", Cluster: "prod", Service: "api",
	}, "api"))

	byFile := map[string]map[string]string{}
	for _, w := range writes {
		vals := map[string]string{}
		for _, p := range w.params {
			vals[p.Name] = p.Value
		}
		byFile[w.name] = vals
	}

	// unused = on the path but not referenced by the TDF, kept with its value.
	require.NotNilf(t, byFile["api.unused.env"], "expected api.unused.env; files: %v", byFile)
	assert.Equal(t, map[string]string{"LEGACY": "old"}, byFile["api.unused.env"])

	// missing = repo-scoped TDF env var absent from the path, empty-valued; the
	// shared secret is excluded.
	missing := byFile["api.missing.env"]
	require.NotNilf(t, missing, "expected api.missing.env; files: %v", byFile)
	v, ok := missing["API_KEY"]
	assert.True(t, ok, "API_KEY should be flagged missing")
	assert.Empty(t, v, "missing entries are empty-valued")
	_, hasShared := missing["SHARED_TOKEN"]
	assert.False(t, hasShared, "shared secret must be excluded from missing")
}

func TestPullByPathWritesEmptyRepoTrioWhenNoParams(t *testing.T) {
	ssm := mocks.NewMockSSMClient(t)
	ssm.EXPECT().GetByPath(mock.Anything, mock.Anything).Return(nil, nil)

	ecs := mocks.NewMockECSClient(t)
	ecs.EXPECT().GetTaskSecrets(mock.Anything, mock.Anything, mock.Anything).Return(nil, []byte("[]"), nil)

	aws := mocks.NewMockAWS(t)
	aws.EXPECT().SSM(mock.Anything, "r").Return(ssm, nil)
	aws.EXPECT().ECS(mock.Anything, "r").Return(ecs, nil)

	env := mocks.NewMockEnvFiles(t)
	written := map[string]int{}
	env.EXPECT().Write(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_, name string, p []awsx.Parameter) error {
			written[name] = len(p)
			return nil
		})

	svc, _ := buildService(t, deps{aws: aws, env: env})
	require.NoError(t, svc.PullByPath(context.Background(), secrets.ParamTarget{Env: "prod", Repo: "api", Region: "r"}, "api"))

	// The per-repo trio is always created, here all empty.
	assert.Equal(t, map[string]int{"api.env": 0, "api.unused.env": 0, "api.missing.env": 0}, written)
}

func TestPullByPathShadowParamIsUnusedNotInMain(t *testing.T) {
	ssm := mocks.NewMockSSMClient(t)
	ssm.EXPECT().GetByPath(mock.Anything, mock.Anything).
		Return([]awsx.Parameter{
			{Name: "DB_HOST", Value: "h"},            // TDF sources it from this path → used
			{Name: "SHARED_KEY", Value: "repo-copy"}, // exists here but TDF sources it from global → shadow
			{Name: "ORPHAN", Value: "stale"},         // not referenced anywhere → orphan
		}, nil)

	ecs := mocks.NewMockECSClient(t)
	ecs.EXPECT().GetTaskSecrets(mock.Anything, "prod", "api").Return([]awsx.TaskSecret{
		{EnvVarName: "DB_HOST", SSMPath: "arn:aws:ssm:r:1:parameter/prod/api/DB_HOST"},
		{EnvVarName: "SHARED_KEY", SSMPath: "arn:aws:ssm:r:1:parameter/prod/global/PLATFORM_KEY"},
	}, []byte("[]"), nil)

	aws := mocks.NewMockAWS(t)
	aws.EXPECT().SSM(mock.Anything, "r").Return(ssm, nil)
	aws.EXPECT().ECS(mock.Anything, "r").Return(ecs, nil)

	env := mocks.NewMockEnvFiles(t)
	written := map[string]map[string]string{}
	env.EXPECT().Write(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_, name string, p []awsx.Parameter) error {
			vals := map[string]string{}
			for _, pp := range p {
				vals[pp.Name] = pp.Value
			}
			written[name] = vals
			return nil
		})

	svc, _ := buildService(t, deps{aws: aws, env: env})
	require.NoError(t, svc.PullByPath(context.Background(), secrets.ParamTarget{
		Env: "prod", Repo: "api", Region: "r", Cluster: "prod", Service: "api",
	}, "api"))

	// Only the param the TDF sources from this path lands in the main file.
	assert.Equal(t, map[string]string{"DB_HOST": "h"}, written["api.env"],
		"the main file holds only params the TDF sources from this path")
	// The shadow (sourced from global) and the orphan are unused, with their values.
	assert.Equal(t, map[string]string{"SHARED_KEY": "repo-copy", "ORPHAN": "stale"}, written["api.unused.env"],
		"a param in SSM here but not sourced from this path by the TDF is unused, not in the main file")
}

func TestPullByPathRelabelsUnusedByTaskDefName(t *testing.T) {
	ssm := mocks.NewMockSSMClient(t)
	ssm.EXPECT().GetByPath(mock.Anything, mock.Anything).
		Return([]awsx.Parameter{{Name: "CACHE_URL", Value: "redis://x"}}, nil)

	// The service exposes CACHE_HOST sourced from a shared param named CACHE_URL.
	// The repo's own CACHE_URL is unused, but should be labeled by the env-var name.
	ecs := mocks.NewMockECSClient(t)
	ecs.EXPECT().GetTaskSecrets(mock.Anything, "prod", "api").Return([]awsx.TaskSecret{
		{EnvVarName: "CACHE_HOST", SSMPath: "arn:aws:ssm:r:1:parameter/prod/global/CACHE_URL"},
	}, []byte("[]"), nil)

	aws := mocks.NewMockAWS(t)
	aws.EXPECT().SSM(mock.Anything, "r").Return(ssm, nil)
	aws.EXPECT().ECS(mock.Anything, "r").Return(ecs, nil)

	env := mocks.NewMockEnvFiles(t)
	written := map[string]map[string]string{}
	env.EXPECT().Write(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_, name string, p []awsx.Parameter) error {
			vals := map[string]string{}
			for _, pp := range p {
				vals[pp.Name] = pp.Value
			}
			written[name] = vals
			return nil
		})

	svc, _ := buildService(t, deps{aws: aws, env: env})
	require.NoError(t, svc.PullByPath(context.Background(), secrets.ParamTarget{
		Env: "prod", Repo: "api", Region: "r", Cluster: "prod", Service: "api",
	}, "api"))

	assert.Equal(t, map[string]string{"CACHE_HOST": "redis://x"}, written["api.unused.env"],
		"an unused param matching a TDF source key should be labeled by the env-var name, value kept")
}

func TestPullByTaskDefGroupsByPrefixAndFillsMissing(t *testing.T) {
	ecs := mocks.NewMockECSClient(t)
	ecs.EXPECT().
		GetTaskSecrets(mock.Anything, "prod", "api").
		Return([]awsx.TaskSecret{
			{EnvVarName: "DB_PASS", SSMPath: "/prod/api/db_pass"},
			{EnvVarName: "API_KEY", SSMPath: "/prod/api/api_key"},        // unresolved by SSM below
			{EnvVarName: "SHARED_TOKEN", SSMPath: "/prod/global/shared"}, // different prefix → own file
		}, []byte("[]"), nil)

	ssm := mocks.NewMockSSMClient(t)
	ssm.EXPECT().
		GetByNames(mock.Anything, mock.Anything).
		Return([]awsx.Parameter{
			{Name: "DB_PASS", Value: "s3cret"},
			{Name: "SHARED_TOKEN", Value: "tok"},
		}, nil)
	// The reconciliation path scan returns exactly the repo-scoped secrets, so the
	// export has nothing unused and nothing missing — only the grouped files.
	ssm.EXPECT().
		GetByPath(mock.Anything, mock.Anything).
		Return([]awsx.Parameter{{Name: "db_pass", Value: "s3cret"}, {Name: "api_key"}}, nil)

	aws := mocks.NewMockAWS(t)
	aws.EXPECT().ECS(mock.Anything, "r").Return(ecs, nil)
	aws.EXPECT().SSM(mock.Anything, "r").Return(ssm, nil)

	env := mocks.NewMockEnvFiles(t)
	var rawWrites []string
	env.EXPECT().WriteRaw(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_, name string, _ []byte) error {
			rawWrites = append(rawWrites, name)
			return nil
		})
	var writes []writeCall
	env.EXPECT().Write(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(dir, name string, p []awsx.Parameter) error {
			writes = append(writes, writeCall{dir, name, p})
			return nil
		})

	svc, cfg := buildService(t, deps{aws: aws, env: env})
	require.NoError(t, svc.PullByTaskDef(context.Background(), secrets.TaskTarget{
		Region: "r", Cluster: "prod", Service: "api", OutputDir: "notifier",
		Env: "prod", Repo: "api",
	}))

	assert.Equal(t, []string{"tdf-secrets.json"}, rawWrites)

	// Params are split into per-prefix files under output/<OutputDir>/.
	wantDir := filepath.Join(cfg.OutputDir(), "notifier")
	byFile := map[string]map[string]string{}
	for _, w := range writes {
		assert.Equalf(t, wantDir, w.dir, "write %q in wrong dir", w.name)
		vals := map[string]string{}
		for _, p := range w.params {
			vals[p.Name] = p.Value
		}
		byFile[w.name] = vals
	}

	// Two grouped files (api.env, global.env) plus the always-present, here-empty
	// reconciliation pair for the repo.
	require.Lenf(t, byFile, 4, "expected api.env, global.env + reconciliation pair, got %v", writes)
	assert.Empty(t, byFile["api.unused.env"], "nothing unused in this fixture")
	assert.Empty(t, byFile["api.missing.env"], "nothing missing in this fixture")
	api := byFile["api.env"]
	require.NotNilf(t, api, "missing api.env; files: %v", byFile)
	assert.Equal(t, "s3cret", api["DB_PASS"], "DB_PASS not resolved")
	v, ok := api["API_KEY"]
	assert.True(t, ok, "expected API_KEY present in api.env")
	assert.Empty(t, v, "expected API_KEY filled empty")
	global := byFile["global.env"]
	require.NotNilf(t, global, "missing global.env; files: %v", byFile)
	assert.Equal(t, "tok", global["SHARED_TOKEN"])
}

func TestPullByTaskDefWritesReconciliation(t *testing.T) {
	ecs := mocks.NewMockECSClient(t)
	ecs.EXPECT().GetTaskSecrets(mock.Anything, "prod", "api").Return([]awsx.TaskSecret{
		{EnvVarName: "DB_PASS", SSMPath: "arn:aws:ssm:r:1:parameter/prod/api/db_pass"},
	}, []byte("[]"), nil)

	ssm := mocks.NewMockSSMClient(t)
	ssm.EXPECT().GetByNames(mock.Anything, mock.Anything).
		Return([]awsx.Parameter{{Name: "DB_PASS", Value: "s3cret"}}, nil)
	// The path holds an extra parameter the TDF never references → unused.
	ssm.EXPECT().GetByPath(mock.Anything, mock.Anything).
		Return([]awsx.Parameter{{Name: "db_pass", Value: "s3cret"}, {Name: "legacy", Value: "old"}}, nil)

	aws := mocks.NewMockAWS(t)
	aws.EXPECT().ECS(mock.Anything, "r").Return(ecs, nil)
	aws.EXPECT().SSM(mock.Anything, "r").Return(ssm, nil)

	env := mocks.NewMockEnvFiles(t)
	env.EXPECT().WriteRaw(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	var writes []writeCall
	env.EXPECT().Write(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(dir, name string, p []awsx.Parameter) error {
			writes = append(writes, writeCall{dir, name, p})
			return nil
		})

	svc, _ := buildService(t, deps{aws: aws, env: env})
	require.NoError(t, svc.PullByTaskDef(context.Background(), secrets.TaskTarget{
		Region: "r", Cluster: "prod", Service: "api", OutputDir: "api", Env: "prod", Repo: "api",
	}))

	var unused *writeCall
	for i := range writes {
		if writes[i].name == "api.unused.env" {
			unused = &writes[i]
		}
	}
	require.NotNilf(t, unused, "expected api.unused.env on the tdf export; writes: %v", writes)
	require.Len(t, unused.params, 1)
	assert.Equal(t, "legacy", unused.params[0].Name)
	assert.Equal(t, "old", unused.params[0].Value, "unused params keep their value")
}

func TestPushParsesAndUploads(t *testing.T) {
	env := mocks.NewMockEnvFiles(t)
	env.EXPECT().Parse(mock.Anything).Return([]awsx.Parameter{{Name: "A"}, {Name: "B"}}, nil)

	ssm := mocks.NewMockSSMClient(t)
	var putDest awsx.ParamPath
	ssm.EXPECT().Put(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, dest awsx.ParamPath, p []awsx.Parameter) (int, []error) {
			putDest = dest
			return len(p), nil
		})
	aws := mocks.NewMockAWS(t)
	aws.EXPECT().SSM(mock.Anything, "r").Return(ssm, nil)

	svc, _ := buildService(t, deps{aws: aws, env: env})
	n, errs := svc.Push(context.Background(), secrets.ParamTarget{Env: "prod", Repo: "api", Region: "r"}, "in.env")
	require.Empty(t, errs)
	assert.Equal(t, 2, n)
	assert.Equal(t, awsx.ParamPath{Env: "prod", Repo: "api"}, putDest)
}

func TestDelegations(t *testing.T) {
	sess := mocks.NewMockSessionClient(t)
	sess.EXPECT().Check(mock.Anything).Return(awsx.Session{AccountID: "123"}, nil)
	sess.EXPECT().ListEnabledRegions(mock.Anything).Return([]string{"us-east-1"}, nil)

	ecs := mocks.NewMockECSClient(t)
	ecs.EXPECT().ListClusters(mock.Anything).Return([]awsx.Cluster{{Name: "c"}}, nil)
	ecs.EXPECT().ListServices(mock.Anything, mock.Anything).Return([]awsx.Service{{Name: "s"}}, nil)
	aws := mocks.NewMockAWS(t)
	aws.EXPECT().ECS(mock.Anything, mock.Anything).Return(ecs, nil)

	creds := mocks.NewMockCredentialStore(t)
	var savedARN, newRegion string
	creds.EXPECT().Save(mock.Anything, mock.Anything).
		RunAndReturn(func(_ awsx.Credentials, arn string) error { savedARN = arn; return nil })
	creds.EXPECT().UpdateRegion(mock.Anything).
		RunAndReturn(func(r string) error { newRegion = r; return nil })

	svc, _ := buildService(t, deps{aws: aws, sess: sess, creds: creds})
	ctx := context.Background()

	s, _ := svc.CheckSession(ctx)
	assert.Equal(t, "123", s.AccountID, "CheckSession did not delegate")
	r, _ := svc.ListRegions(ctx)
	assert.Len(t, r, 1, "ListRegions did not delegate")
	c, _ := svc.ListClusters(ctx, "r")
	assert.Len(t, c, 1, "ListClusters did not delegate")
	svcs, _ := svc.ListServices(ctx, "r", "c")
	assert.Len(t, svcs, 1, "ListServices did not delegate")
	require.NoError(t, svc.SaveCredentials(awsx.Credentials{}, "arn:x"))
	assert.Equal(t, "arn:x", savedARN, "SaveCredentials did not delegate")
	require.NoError(t, svc.UpdateRegion("us-west-2"))
	assert.Equal(t, "us-west-2", newRegion, "UpdateRegion did not delegate")
}

func TestCheckSessionPropagatesError(t *testing.T) {
	sess := mocks.NewMockSessionClient(t)
	sess.EXPECT().Check(mock.Anything).Return(awsx.Session{}, errors.New("expired"))

	svc, _ := buildService(t, deps{sess: sess})
	_, err := svc.CheckSession(context.Background())
	assert.Error(t, err, "expected error to propagate")
}
