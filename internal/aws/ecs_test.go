package aws

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type mockECS struct {
	ECSAPI
	ListClustersFunc           func(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListServicesFunc           func(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	DescribeServicesFunc       func(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	DescribeTaskDefinitionFunc func(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
}

func (m *mockECS) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return m.ListClustersFunc(ctx, params, optFns...)
}
func (m *mockECS) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	return m.ListServicesFunc(ctx, params, optFns...)
}
func (m *mockECS) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	return m.DescribeServicesFunc(ctx, params, optFns...)
}
func (m *mockECS) DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	return m.DescribeTaskDefinitionFunc(ctx, params, optFns...)
}

type mockCW struct {
	CWAPI
	FilterLogEventsFunc func(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

func (m *mockCW) FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	return m.FilterLogEventsFunc(ctx, params, optFns...)
}

func TestListClusters(t *testing.T) {
	mock := &mockECS{
		ListClustersFunc: func(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{ClusterArns: []string{"arn:aws:ecs:r:a:cluster/c1"}}, nil
		},
	}
	client := &ECSClient{ecsClient: mock}
	res, _ := client.ListClusters(context.Background())
	if len(res) != 1 || res[0] != "c1" {
		t.Error("wrong result")
	}
}

func TestListServices(t *testing.T) {
	mock := &mockECS{
		ListServicesFunc: func(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
			return &ecs.ListServicesOutput{ServiceArns: []string{"arn:aws:ecs:r:a:service/c1/s1"}}, nil
		},
	}
	client := &ECSClient{ecsClient: mock}
	res, _ := client.ListServices(context.Background(), "c1")
	if len(res) != 1 || res[0] != "s1" {
		t.Error("wrong result")
	}
}

func TestGetTaskDefinitionExtended(t *testing.T) {
	mock := &mockECS{
		DescribeServicesFunc: func(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
			if strings.Contains(params.Services[0], "missing") {
				return &ecs.DescribeServicesOutput{Services: []types.Service{}}, nil
			}
			return &ecs.DescribeServicesOutput{Services: []types.Service{{TaskDefinition: aws.String("td1")}}}, nil
		},
		DescribeTaskDefinitionFunc: func(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
			return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &types.TaskDefinition{TaskDefinitionArn: aws.String("td1")}}, nil
		},
	}
	client := &ECSClient{ecsClient: mock}
	_, _, err := client.GetTaskDefinition(context.Background(), "c1", "s1")
	if err != nil {
		t.Error(err)
	}

	_, _, err = client.GetTaskDefinition(context.Background(), "c1", "missing")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetServiceLogsExtended(t *testing.T) {
	mECS := &mockECS{
		DescribeServicesFunc: func(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
			return &ecs.DescribeServicesOutput{Services: []types.Service{{TaskDefinition: aws.String("td1")}}}, nil
		},
		DescribeTaskDefinitionFunc: func(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
			return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &types.TaskDefinition{
				ContainerDefinitions: []types.ContainerDefinition{{
					LogConfiguration: &types.LogConfiguration{
						LogDriver: types.LogDriverAwslogs,
						Options:   map[string]string{"awslogs-group": "g1"},
					},
				}},
			}}, nil
		},
	}
	mCW := &mockCW{
		FilterLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
			return &cloudwatchlogs.FilterLogEventsOutput{Events: []cwtypes.FilteredLogEvent{{Message: aws.String("msg1"), Timestamp: aws.Int64(123000)}}}, nil
		},
	}
	client := &ECSClient{ecsClient: mECS, cwClient: mCW}
	logs, _ := client.GetServiceLogs(context.Background(), "c1", "s1", 10)
	if len(logs) != 1 {
		t.Error("failed")
	}
}

func TestNewECSClientError(t *testing.T) {
	BaseDir = "non_existent"
	_, err := NewECSClient(context.Background(), "")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetTaskDefinitionError(t *testing.T) {
	mock := &mockECS{
		DescribeServicesFunc: func(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
			return nil, fmt.Errorf("error")
		},
	}
	client := &ECSClient{ecsClient: mock}
	_, _, err := client.GetTaskDefinition(context.Background(), "c1", "s1")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetServiceLogsError(t *testing.T) {
	mock := &mockECS{
		DescribeServicesFunc: func(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
			return nil, fmt.Errorf("error")
		},
	}
	client := &ECSClient{ecsClient: mock}
	_, err := client.GetServiceLogs(context.Background(), "c1", "s1", 10)
	if err == nil {
		t.Error("expected error")
	}
}
