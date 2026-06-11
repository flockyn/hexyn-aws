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

func TestPullByPathWritesEnvFile(t *testing.T) {
	ssm := mocks.NewMockSSMClient(t)
	ssm.EXPECT().
		GetByPath(mock.Anything, awsx.ParamPath{Env: "prod", Repo: "api"}).
		Return([]awsx.Parameter{{Name: "DB_HOST", Value: "x"}}, nil)
	aws := mocks.NewMockAWS(t)
	aws.EXPECT().SSM(mock.Anything, "ap-southeast-3").Return(ssm, nil)

	env := mocks.NewMockEnvFiles(t)
	var got writeCall
	env.EXPECT().Write(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(dir, name string, p []awsx.Parameter) error {
			got = writeCall{dir, name, p}
			return nil
		})

	svc, cfg := buildService(t, deps{aws: aws, env: env})
	require.NoError(t, svc.PullByPath(context.Background(), secrets.ParamTarget{Env: "prod", Repo: "api", Region: "ap-southeast-3"}, "api"))
	assert.Equal(t, filepath.Join(cfg.OutputDir(), "api"), got.dir)
	assert.Equal(t, "ps.env", got.name)
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
		Region: "r", Cluster: "prod", Service: "api", OutputDir: "wec-notification",
	}))

	assert.Equal(t, []string{"tdf-secrets.json"}, rawWrites)

	// Params are split into per-prefix files under output/<OutputDir>/.
	wantDir := filepath.Join(cfg.OutputDir(), "wec-notification")
	byFile := map[string]map[string]string{}
	for _, w := range writes {
		assert.Equalf(t, wantDir, w.dir, "write %q in wrong dir", w.name)
		vals := map[string]string{}
		for _, p := range w.params {
			vals[p.Name] = p.Value
		}
		byFile[w.name] = vals
	}

	require.Lenf(t, byFile, 2, "expected 2 grouped files (api.env, global.env), got %v", writes)
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
