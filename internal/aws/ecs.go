package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type ECSAPI interface {
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
}

type CWAPI interface {
	FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

type ECSClient struct {
	ecsClient ECSAPI
	cwClient  CWAPI
}

func NewECSClient(ctx context.Context, region string) (*ECSClient, error) {
	cfg, _, _, _, err := LoadAWSConfig(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	return &ECSClient{
		ecsClient: ecs.NewFromConfig(cfg),
		cwClient:  cloudwatchlogs.NewFromConfig(cfg),
	}, nil
}

func (e *ECSClient) ListClusters(ctx context.Context) ([]string, error) {
	var clusters []string
	out, err := e.ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, err
	}
	for _, arn := range out.ClusterArns {
		parts := strings.Split(arn, "/")
		clusters = append(clusters, parts[len(parts)-1])
	}
	return clusters, nil
}

func (e *ECSClient) ListServices(ctx context.Context, cluster string) ([]string, error) {
	var services []string
	out, err := e.ecsClient.ListServices(ctx, &ecs.ListServicesInput{
		Cluster: aws.String(cluster),
	})
	if err != nil {
		return nil, err
	}
	for _, arn := range out.ServiceArns {
		parts := strings.Split(arn, "/")
		services = append(services, parts[len(parts)-1])
	}
	return services, nil
}

func (e *ECSClient) GetTaskDefinition(ctx context.Context, cluster, service string) (*ecstypes.TaskDefinition, []byte, error) {
	svcOut, err := e.ecsClient.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{service},
	})
	if err != nil {
		return nil, nil, err
	}
	if len(svcOut.Services) == 0 {
		return nil, nil, fmt.Errorf("service not found")
	}

	tdArn := svcOut.Services[0].TaskDefinition
	tdOut, err := e.ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: tdArn,
	})
	if err != nil {
		return nil, nil, err
	}

	jsonData, err := json.MarshalIndent(tdOut.TaskDefinition, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal task definition: %w", err)
	}

	return tdOut.TaskDefinition, jsonData, nil
}

func (e *ECSClient) GetTaskSecrets(td *ecstypes.TaskDefinition) map[string]string {
	secrets := make(map[string]string)
	for _, container := range td.ContainerDefinitions {
		for _, secret := range container.Secrets {
			secrets[*secret.Name] = *secret.ValueFrom
		}
	}
	return secrets
}

func (e *ECSClient) GetServiceLogs(ctx context.Context, cluster, service string, limit int32) ([]string, error) {
	svcOut, err := e.ecsClient.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{service},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe service: %w", err)
	}
	if len(svcOut.Services) == 0 {
		return nil, fmt.Errorf("service %s not found", service)
	}

	taskDefArn := svcOut.Services[0].TaskDefinition
	tdOut, err := e.ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: taskDefArn,
	})
	if err != nil || tdOut.TaskDefinition == nil || len(tdOut.TaskDefinition.ContainerDefinitions) == 0 {
		return nil, fmt.Errorf("failed to describe task definition: %w", err)
	}

	container := tdOut.TaskDefinition.ContainerDefinitions[0]
	logConfig := container.LogConfiguration
	if logConfig == nil || logConfig.LogDriver != "awslogs" {
		return nil, fmt.Errorf("not using awslogs")
	}

	logGroup := logConfig.Options["awslogs-group"]
	logOut, err := e.cwClient.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: aws.String(logGroup),
		Limit:        aws.Int32(limit),
		StartTime:    aws.Int64(time.Now().Add(-1*time.Hour).Unix() * 1000),
	})
	if err != nil {
		return nil, err
	}

	var logs []string
	for _, event := range logOut.Events {
		logs = append(logs, fmt.Sprintf("[%s] %s",
			time.Unix(*event.Timestamp/1000, 0).Format("15:04:05"),
			*event.Message))
	}

	return logs, nil
}
