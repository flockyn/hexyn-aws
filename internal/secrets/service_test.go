package secrets

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/config"
)

// --- fakes ---------------------------------------------------------------

type fakeSSM struct {
	byPath  []awsx.Parameter
	byNames []awsx.Parameter
	putDest awsx.ParamPath
	putN    int
}

func (f *fakeSSM) GetByPath(context.Context, awsx.ParamPath) ([]awsx.Parameter, error) {
	return f.byPath, nil
}
func (f *fakeSSM) GetByNames(context.Context, map[string]string) ([]awsx.Parameter, error) {
	return f.byNames, nil
}
func (f *fakeSSM) Put(_ context.Context, dest awsx.ParamPath, p []awsx.Parameter) (int, []error) {
	f.putDest = dest
	f.putN = len(p)
	return len(p), nil
}

type fakeECS struct {
	clusters []awsx.Cluster
	services []awsx.Service
	secrets  []awsx.TaskSecret
	raw      []byte
}

func (f *fakeECS) ListClusters(context.Context) ([]awsx.Cluster, error) { return f.clusters, nil }
func (f *fakeECS) ListServices(context.Context, string) ([]awsx.Service, error) {
	return f.services, nil
}
func (f *fakeECS) GetTaskSecrets(context.Context, string, string) ([]awsx.TaskSecret, []byte, error) {
	return f.secrets, f.raw, nil
}

type fakeAWS struct {
	ssm *fakeSSM
	ecs *fakeECS
}

func (f fakeAWS) SSM(context.Context, string) (SSMClient, error) { return f.ssm, nil }
func (f fakeAWS) ECS(context.Context, string) (ECSClient, error) { return f.ecs, nil }

type writeCall struct {
	dir, name string
	params    []awsx.Parameter
}

type fakeEnv struct {
	parse     []awsx.Parameter
	writes    []writeCall
	rawWrites []string // names
}

func (f *fakeEnv) Parse(string) ([]awsx.Parameter, error) { return f.parse, nil }
func (f *fakeEnv) Write(dir, name string, p []awsx.Parameter) error {
	f.writes = append(f.writes, writeCall{dir, name, p})
	return nil
}
func (f *fakeEnv) WriteRaw(_, name string, _ []byte) error {
	f.rawWrites = append(f.rawWrites, name)
	return nil
}

type fakeCreds struct {
	savedARN  string
	newRegion string
}

func (f *fakeCreds) Save(_ awsx.Credentials, arn string) error { f.savedARN = arn; return nil }
func (f *fakeCreds) UpdateRegion(r string) error               { f.newRegion = r; return nil }

type fakeSession struct {
	session awsx.Session
	regions []string
	err     error
}

func (f fakeSession) Check(context.Context) (awsx.Session, error) { return f.session, f.err }
func (f fakeSession) ListEnabledRegions(context.Context) ([]string, error) {
	return f.regions, nil
}

// newService wires a Service over the given fakes with a local config provider.
func newService(aws AWS, env EnvFiles, creds CredentialStore, sess SessionClient) (*Service, *config.Provider) {
	cfg := config.New(true) // OutputDir == ".hexyn-aws/output"
	return New(Deps{Cfg: cfg, Creds: creds, Session: sess, Env: env, AWS: aws}), cfg
}

// --- tests ---------------------------------------------------------------

func TestPullByPathWritesEnvFile(t *testing.T) {
	ssm := &fakeSSM{byPath: []awsx.Parameter{{Name: "DB_HOST", Value: "x"}}}
	env := &fakeEnv{}
	svc, cfg := newService(fakeAWS{ssm: ssm}, env, &fakeCreds{}, fakeSession{})

	if err := svc.PullByPath(context.Background(), ParamTarget{Env: "prod", Repo: "api", Region: "ap-southeast-3"}, "api"); err != nil {
		t.Fatal(err)
	}
	if len(env.writes) != 1 {
		t.Fatalf("expected 1 write, got %d", len(env.writes))
	}
	w := env.writes[0]
	wantDir := filepath.Join(cfg.OutputDir(), "api")
	if w.dir != wantDir || w.name != "ps.env" {
		t.Errorf("unexpected write target: %s/%s (want %s/ps.env)", w.dir, w.name, wantDir)
	}
}

func TestPullByTaskDefGroupsByPrefixAndFillsMissing(t *testing.T) {
	ecs := &fakeECS{
		secrets: []awsx.TaskSecret{
			{EnvVarName: "DB_PASS", SSMPath: "/prod/api/db_pass"},
			{EnvVarName: "API_KEY", SSMPath: "/prod/api/api_key"},        // unresolved by SSM below
			{EnvVarName: "SHARED_TOKEN", SSMPath: "/prod/global/shared"}, // different prefix → own file
		},
		raw: []byte("[]"),
	}
	ssm := &fakeSSM{byNames: []awsx.Parameter{
		{Name: "DB_PASS", Value: "s3cret"},
		{Name: "SHARED_TOKEN", Value: "tok"},
	}}
	env := &fakeEnv{}
	svc, cfg := newService(fakeAWS{ssm: ssm, ecs: ecs}, env, &fakeCreds{}, fakeSession{})

	err := svc.PullByTaskDef(context.Background(), TaskTarget{
		Region: "r", Cluster: "prod", Service: "api", OutputDir: "wec-notification",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(env.rawWrites) != 1 || env.rawWrites[0] != "tdf-secrets.json" {
		t.Errorf("expected tdf-secrets.json raw write, got %v", env.rawWrites)
	}

	// Params are split into per-prefix files under output/<OutputDir>/.
	wantDir := filepath.Join(cfg.OutputDir(), "wec-notification")
	byFile := map[string]map[string]string{}
	for _, w := range env.writes {
		if w.dir != wantDir {
			t.Errorf("write %q in wrong dir %q (want %q)", w.name, w.dir, wantDir)
		}
		vals := map[string]string{}
		for _, p := range w.params {
			vals[p.Name] = p.Value
		}
		byFile[w.name] = vals
	}

	if len(byFile) != 2 {
		t.Fatalf("expected 2 grouped files (api.env, global.env), got %d: %v", len(byFile), env.writes)
	}
	api, ok := byFile["api.env"]
	if !ok {
		t.Fatalf("missing api.env; files: %v", byFile)
	}
	if api["DB_PASS"] != "s3cret" {
		t.Errorf("DB_PASS not resolved: %q", api["DB_PASS"])
	}
	if v, ok := api["API_KEY"]; !ok || v != "" {
		t.Errorf("expected API_KEY filled empty in api.env, got %q (present=%v)", v, ok)
	}
	global, ok := byFile["global.env"]
	if !ok {
		t.Fatalf("missing global.env; files: %v", byFile)
	}
	if global["SHARED_TOKEN"] != "tok" {
		t.Errorf("SHARED_TOKEN not in global.env: %q", global["SHARED_TOKEN"])
	}
}

func TestPushParsesAndUploads(t *testing.T) {
	env := &fakeEnv{parse: []awsx.Parameter{{Name: "A"}, {Name: "B"}}}
	ssm := &fakeSSM{}
	svc, _ := newService(fakeAWS{ssm: ssm}, env, &fakeCreds{}, fakeSession{})

	n, errs := svc.Push(context.Background(), ParamTarget{Env: "prod", Repo: "api", Region: "r"}, "in.env")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if n != 2 {
		t.Errorf("expected 2 uploaded, got %d", n)
	}
	if ssm.putDest != (awsx.ParamPath{Env: "prod", Repo: "api"}) {
		t.Errorf("unexpected put dest: %+v", ssm.putDest)
	}
}

func TestDelegations(t *testing.T) {
	creds := &fakeCreds{}
	sess := fakeSession{session: awsx.Session{AccountID: "123"}, regions: []string{"us-east-1"}}
	ecs := &fakeECS{clusters: []awsx.Cluster{{Name: "c"}}, services: []awsx.Service{{Name: "s"}}}
	svc, _ := newService(fakeAWS{ecs: ecs, ssm: &fakeSSM{}}, &fakeEnv{}, creds, sess)
	ctx := context.Background()

	if s, _ := svc.CheckSession(ctx); s.AccountID != "123" {
		t.Error("CheckSession did not delegate")
	}
	if r, _ := svc.ListRegions(ctx); len(r) != 1 {
		t.Error("ListRegions did not delegate")
	}
	if c, _ := svc.ListClusters(ctx, "r"); len(c) != 1 {
		t.Error("ListClusters did not delegate")
	}
	if s, _ := svc.ListServices(ctx, "r", "c"); len(s) != 1 {
		t.Error("ListServices did not delegate")
	}
	if err := svc.SaveCredentials(awsx.Credentials{}, "arn:x"); err != nil || creds.savedARN != "arn:x" {
		t.Error("SaveCredentials did not delegate")
	}
	if err := svc.UpdateRegion("us-west-2"); err != nil || creds.newRegion != "us-west-2" {
		t.Error("UpdateRegion did not delegate")
	}
}

func TestCheckSessionPropagatesError(t *testing.T) {
	svc, _ := newService(fakeAWS{ssm: &fakeSSM{}}, &fakeEnv{}, &fakeCreds{}, fakeSession{err: errors.New("expired")})
	if _, err := svc.CheckSession(context.Background()); err == nil {
		t.Fatal("expected error to propagate")
	}
}
