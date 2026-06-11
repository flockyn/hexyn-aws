package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mocks "hexyn-aws/test/mocks/service"
)

func TestECSListClustersStripsArn(t *testing.T) {
	api := mocks.NewMockECSAPI(t)
	api.EXPECT().ListClusters(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{ClusterArns: []string{
				"arn:aws:ecs:ap-southeast-3:123:cluster/prod-cluster",
			}}, nil
		})
	e := &ECS{api: api}

	got, err := e.ListClusters(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "prod-cluster", got[0].Name)
}

func TestECSListClustersPaginates(t *testing.T) {
	var calls int
	api := mocks.NewMockECSAPI(t)
	api.EXPECT().ListClusters(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, in *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
			calls++
			if in.NextToken == nil {
				return &ecs.ListClustersOutput{
					ClusterArns: []string{"arn:aws:ecs:r:1:cluster/a", "arn:aws:ecs:r:1:cluster/b"},
					NextToken:   aws.String("page2"),
				}, nil
			}
			return &ecs.ListClustersOutput{ClusterArns: []string{"arn:aws:ecs:r:1:cluster/c"}}, nil
		})
	e := &ECS{api: api}

	got, err := e.ListClusters(context.Background())
	require.NoError(t, err)
	assert.Len(t, got, 3, "expected 3 clusters across pages")
	assert.Equal(t, 2, calls, "expected 2 paginated calls")
}

func TestECSListServicesPaginates(t *testing.T) {
	var calls int
	api := mocks.NewMockECSAPI(t)
	api.EXPECT().ListServices(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, in *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
			calls++
			if in.NextToken == nil {
				arns := make([]string, 0, 10)
				for i := range 10 {
					arns = append(arns, fmt.Sprintf("arn:aws:ecs:r:1:service/cluster/svc-%d", i))
				}
				return &ecs.ListServicesOutput{ServiceArns: arns, NextToken: aws.String("page2")}, nil
			}
			return &ecs.ListServicesOutput{ServiceArns: []string{
				"arn:aws:ecs:r:1:service/cluster/svc-10",
				"arn:aws:ecs:r:1:service/cluster/svc-11",
			}}, nil
		})
	e := &ECS{api: api}

	got, err := e.ListServices(context.Background(), "cluster")
	require.NoError(t, err)
	require.Len(t, got, 12, "expected 12 services across pages")
	assert.Equal(t, "svc-11", got[11].Name)
	assert.Equal(t, 2, calls, "expected 2 paginated calls")
}

func TestECSGetTaskSecrets(t *testing.T) {
	api := mocks.NewMockECSAPI(t)
	api.EXPECT().DescribeServices(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
			return &ecs.DescribeServicesOutput{Services: []ecstypes.Service{
				{TaskDefinition: aws.String("td:1")},
			}}, nil
		})
	api.EXPECT().DescribeTaskDefinition(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
			return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{{
					Secrets: []ecstypes.Secret{
						{Name: aws.String("DB_PASS"), ValueFrom: aws.String("/prod/api/db_pass")},
					},
				}},
			}}, nil
		})
	e := &ECS{api: api}

	secrets, raw, err := e.GetTaskSecrets(context.Background(), "prod-cluster", "api")
	require.NoError(t, err)
	require.Len(t, secrets, 1)
	assert.Equal(t, "DB_PASS", secrets[0].EnvVarName)
	assert.Equal(t, "/prod/api/db_pass", secrets[0].SSMPath)
	assert.NotEmpty(t, raw, "expected non-empty raw JSON")
}

func TestECSGetTaskSecretsServiceNotFound(t *testing.T) {
	api := mocks.NewMockECSAPI(t)
	api.EXPECT().DescribeServices(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
			return &ecs.DescribeServicesOutput{}, nil
		})
	e := &ECS{api: api}

	_, _, err := e.GetTaskSecrets(context.Background(), "c", "missing")
	assert.Error(t, err, "expected error for missing service")
}
