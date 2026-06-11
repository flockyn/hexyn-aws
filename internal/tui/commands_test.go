package tui

import (
	"context"
	"strings"
	"testing"

	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/config"
	"hexyn-aws/internal/secrets"
)

// --- fakes implementing the secrets ports -------------------------------

type fakeSession struct {
	session awsx.Session
	regions []string
}

func (f fakeSession) Check(context.Context) (awsx.Session, error)          { return f.session, nil }
func (f fakeSession) ListEnabledRegions(context.Context) ([]string, error) { return f.regions, nil }

type fakeCreds struct{ region string }

func (f *fakeCreds) Save(awsx.Credentials, string) error { return nil }
func (f *fakeCreds) UpdateRegion(r string) error         { f.region = r; return nil }

type fakeSSM struct{ params []awsx.Parameter }

func (f fakeSSM) GetByPath(context.Context, awsx.ParamPath) ([]awsx.Parameter, error) {
	return f.params, nil
}
func (f fakeSSM) GetByNames(context.Context, map[string]string) ([]awsx.Parameter, error) {
	return nil, nil
}
func (f fakeSSM) Put(context.Context, awsx.ParamPath, []awsx.Parameter) (int, []error) {
	return 0, nil
}

type fakeECS struct{ clusters []awsx.Cluster }

func (f fakeECS) ListClusters(context.Context) ([]awsx.Cluster, error)         { return f.clusters, nil }
func (f fakeECS) ListServices(context.Context, string) ([]awsx.Service, error) { return nil, nil }
func (f fakeECS) GetTaskSecrets(context.Context, string, string) ([]awsx.TaskSecret, []byte, error) {
	return nil, nil, nil
}

type fakeAWS struct {
	ssm fakeSSM
	ecs fakeECS
}

func (f fakeAWS) SSM(context.Context, string) (secrets.SSMClient, error) { return f.ssm, nil }
func (f fakeAWS) ECS(context.Context, string) (secrets.ECSClient, error) { return f.ecs, nil }

type fakeEnv struct{}

func (fakeEnv) Parse(string) ([]awsx.Parameter, error)       { return nil, nil }
func (fakeEnv) Write(string, string, []awsx.Parameter) error { return nil }
func (fakeEnv) WriteRaw(string, string, []byte) error        { return nil }

// runnerWith builds a commandRunner over a real Service, defaulting any
// dependency the test does not care about.
func runnerWith(d secrets.Deps) *commandRunner {
	if d.Cfg == nil {
		d.Cfg = config.New(true)
	}
	if d.Creds == nil {
		d.Creds = &fakeCreds{}
	}
	if d.Session == nil {
		d.Session = fakeSession{}
	}
	if d.Env == nil {
		d.Env = fakeEnv{}
	}
	if d.AWS == nil {
		d.AWS = fakeAWS{}
	}
	return &commandRunner{svc: secrets.New(d)}
}

// --- tests --------------------------------------------------------------

func TestCheckSessionCommandEmitsSessionMsg(t *testing.T) {
	cr := runnerWith(secrets.Deps{Session: fakeSession{session: awsx.Session{AccountID: "123"}}})

	msg := cr.checkSession()()
	got, ok := msg.(sessionMsg)
	if !ok {
		t.Fatalf("expected sessionMsg, got %T", msg)
	}
	if got.session.AccountID != "123" {
		t.Errorf("session not carried through: %+v", got.session)
	}
}

func TestListRegionsCommandBuildsItems(t *testing.T) {
	cr := runnerWith(secrets.Deps{Session: fakeSession{regions: []string{"ap-southeast-3", "us-east-1"}}})

	got, ok := cr.listRegions()().(listMsg)
	if !ok {
		t.Fatal("expected listMsg")
	}
	if len(got.items) != 2 || got.next != stateSelectRegion {
		t.Fatalf("unexpected listMsg: %+v", got)
	}
	if got.items[0].(item).title != "ap-southeast-3" {
		t.Errorf("first region item wrong: %+v", got.items[0])
	}
}

func TestListClustersEmptyIsError(t *testing.T) {
	cr := runnerWith(secrets.Deps{AWS: fakeAWS{ecs: fakeECS{clusters: nil}}})

	got := cr.listClusters("ap-southeast-3")().(listMsg)
	if got.err == nil {
		t.Fatal("expected an error when no clusters are found")
	}
}

func TestUpdateRegionCommandConfirms(t *testing.T) {
	cr := runnerWith(secrets.Deps{Creds: &fakeCreds{}})

	got := cr.updateRegion("ap-southeast-3")().(resultMsg)
	if got.err != nil || got.message != "Region updated to ap-southeast-3" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestGetByPathCommandReportsSuccess(t *testing.T) {
	cr := runnerWith(secrets.Deps{AWS: fakeAWS{ssm: fakeSSM{params: []awsx.Parameter{{Name: "X"}}}}})

	target := secrets.ParamTarget{Env: "prod", Repo: "api", Region: "ap-southeast-3"}
	got := cr.getByPath(target, "api")().(resultMsg)
	if got.err != nil {
		t.Fatalf("unexpected error: %v", got.err)
	}
	if !strings.Contains(got.message, "Successfully exported") {
		t.Errorf("unexpected message: %q", got.message)
	}
}
