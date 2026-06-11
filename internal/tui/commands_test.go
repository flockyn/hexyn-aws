package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/secrets"
	mocks "hexyn-aws/test/mocks/secrets"
)

func TestCheckSessionCommandEmitsSessionMsg(t *testing.T) {
	sess := mocks.NewMockSessionClient(t)
	sess.EXPECT().Check(mock.Anything).Return(awsx.Session{AccountID: "123"}, nil)
	cr := runnerWith(t, secrets.Deps{Session: sess})

	msg := cr.checkSession()()
	got, ok := msg.(sessionMsg)
	require.Truef(t, ok, "expected sessionMsg, got %T", msg)
	assert.Equal(t, "123", got.session.AccountID)
}

func TestListRegionsCommandBuildsItems(t *testing.T) {
	sess := mocks.NewMockSessionClient(t)
	sess.EXPECT().ListEnabledRegions(mock.Anything).Return([]string{"ap-southeast-3", "us-east-1"}, nil)
	cr := runnerWith(t, secrets.Deps{Session: sess})

	got, ok := cr.listRegions()().(listMsg)
	require.True(t, ok, "expected listMsg")
	require.Len(t, got.items, 2)
	assert.Equal(t, stateSelectRegion, got.next)
	assert.Equal(t, "ap-southeast-3", got.items[0].(item).title)
}

func TestListClustersEmptyIsError(t *testing.T) {
	ecs := mocks.NewMockECSClient(t)
	ecs.EXPECT().ListClusters(mock.Anything).Return(nil, nil)
	aws := mocks.NewMockAWS(t)
	aws.EXPECT().ECS(mock.Anything, "ap-southeast-3").Return(ecs, nil)
	cr := runnerWith(t, secrets.Deps{AWS: aws})

	got := cr.listClusters("ap-southeast-3")().(listMsg)
	assert.Error(t, got.err, "expected an error when no clusters are found")
}

func TestUpdateRegionCommandConfirms(t *testing.T) {
	creds := mocks.NewMockCredentialStore(t)
	creds.EXPECT().UpdateRegion("ap-southeast-3").Return(nil)
	cr := runnerWith(t, secrets.Deps{Creds: creds})

	got := cr.updateRegion("ap-southeast-3")().(resultMsg)
	require.NoError(t, got.err)
	assert.Equal(t, "Region updated to ap-southeast-3", got.message)
}

func TestGetByPathCommandReportsSuccess(t *testing.T) {
	ssm := mocks.NewMockSSMClient(t)
	ssm.EXPECT().GetByPath(mock.Anything, mock.Anything).Return([]awsx.Parameter{{Name: "X"}}, nil)
	aws := mocks.NewMockAWS(t)
	aws.EXPECT().SSM(mock.Anything, "ap-southeast-3").Return(ssm, nil)
	env := mocks.NewMockEnvFiles(t)
	env.EXPECT().Write(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	cr := runnerWith(t, secrets.Deps{AWS: aws, Env: env})

	target := secrets.ParamTarget{Env: "prod", Repo: "api", Region: "ap-southeast-3"}
	got := cr.getByPath(target, "api")().(resultMsg)
	require.NoError(t, got.err)
	assert.Contains(t, got.message, "Successfully exported")
}
