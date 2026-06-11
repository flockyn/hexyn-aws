package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"hexyn-aws/internal/awsx"
)

// ecsAPI is the subset of the AWS ECS client used here (enables test mocking).
type ecsAPI interface {
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
}

// ECS is a region-scoped client over AWS ECS.
type ECS struct {
	api ecsAPI
}

// NewECS builds an ECS client configured for the given region.
func NewECS(ctx context.Context, creds awsx.CredentialLoader, region string) (*ECS, error) {
	cfg, err := awsx.BuildConfig(ctx, creds, region)
	if err != nil {
		return nil, fmt.Errorf("unable to build AWS config for region %s: %w", region, err)
	}
	return &ECS{api: ecs.NewFromConfig(cfg)}, nil
}

// listMaxResults is the largest page AWS ECS allows for the List* calls; using
// it minimises the number of paginated round-trips.
const listMaxResults = 100

// ListClusters returns all ECS cluster names in the region, following pagination
// so the result is not capped at a single page.
func (e *ECS) ListClusters(ctx context.Context) ([]awsx.Cluster, error) {
	var clusters []awsx.Cluster
	var next *string
	for {
		out, err := e.api.ListClusters(ctx, &ecs.ListClustersInput{
			MaxResults: aws.Int32(listMaxResults),
			NextToken:  next,
		})
		if err != nil {
			return nil, err
		}
		for _, arn := range out.ClusterArns {
			clusters = append(clusters, awsx.Cluster{Name: e.lastArnSegment(arn)})
		}
		if out.NextToken == nil {
			return clusters, nil
		}
		next = out.NextToken
	}
}

// ListServices returns all ECS service names within the given cluster, following
// pagination (the ECS API defaults to 10 services per page).
func (e *ECS) ListServices(ctx context.Context, cluster string) ([]awsx.Service, error) {
	var services []awsx.Service
	var next *string
	for {
		out, err := e.api.ListServices(ctx, &ecs.ListServicesInput{
			Cluster:    aws.String(cluster),
			MaxResults: aws.Int32(listMaxResults),
			NextToken:  next,
		})
		if err != nil {
			return nil, err
		}
		for _, arn := range out.ServiceArns {
			services = append(services, awsx.Service{Name: e.lastArnSegment(arn)})
		}
		if out.NextToken == nil {
			return services, nil
		}
		next = out.NextToken
	}
}

// rawSecret mirrors the container secret shape for JSON serialisation.
type rawSecret struct {
	Name      string `json:"name"`
	ValueFrom string `json:"valueFrom"`
}

// GetTaskSecrets resolves the active task definition for a service and returns the
// env-var → SSM-path mapping plus the raw JSON of the secrets array (tdf-secrets.json).
func (e *ECS) GetTaskSecrets(ctx context.Context, cluster, service string) ([]awsx.TaskSecret, []byte, error) {
	svcOut, err := e.api.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{service},
	})
	if err != nil {
		return nil, nil, err
	}
	if len(svcOut.Services) == 0 {
		return nil, nil, fmt.Errorf("service %q not found in cluster %q", service, cluster)
	}

	tdOut, err := e.api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: svcOut.Services[0].TaskDefinition,
	})
	if err != nil {
		return nil, nil, err
	}
	if tdOut.TaskDefinition == nil {
		return nil, nil, fmt.Errorf("task definition not found for service %q", service)
	}

	var secrets []awsx.TaskSecret
	var raw []rawSecret
	for _, container := range tdOut.TaskDefinition.ContainerDefinitions {
		for _, s := range container.Secrets {
			secrets = append(secrets, awsx.TaskSecret{EnvVarName: aws.ToString(s.Name), SSMPath: aws.ToString(s.ValueFrom)})
			raw = append(raw, rawSecret{Name: aws.ToString(s.Name), ValueFrom: aws.ToString(s.ValueFrom)})
		}
	}

	jsonBytes, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal task secrets: %w", err)
	}
	return secrets, jsonBytes, nil
}

// lastArnSegment returns the trailing name component of an ARN. The receiver is
// unused; it keeps the helper grouped on ECS.
func (*ECS) lastArnSegment(arn string) string {
	parts := strings.Split(arn, "/")
	return parts[len(parts)-1]
}
