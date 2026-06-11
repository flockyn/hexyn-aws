package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type fakeECS struct {
	describeServices func(*ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error)
	describeTaskDef  func(*ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error)
	listClusters     func(*ecs.ListClustersInput) (*ecs.ListClustersOutput, error)
	listServices     func(*ecs.ListServicesInput) (*ecs.ListServicesOutput, error)
}

func (f *fakeECS) DescribeServices(_ context.Context, in *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	return f.describeServices(in)
}

func (f *fakeECS) DescribeTaskDefinition(_ context.Context, in *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	return f.describeTaskDef(in)
}

func (f *fakeECS) ListClusters(_ context.Context, in *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return f.listClusters(in)
}

func (f *fakeECS) ListServices(_ context.Context, in *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	return f.listServices(in)
}

func TestECSListClustersStripsArn(t *testing.T) {
	fake := &fakeECS{listClusters: func(*ecs.ListClustersInput) (*ecs.ListClustersOutput, error) {
		return &ecs.ListClustersOutput{ClusterArns: []string{
			"arn:aws:ecs:ap-southeast-3:123:cluster/prod-cluster",
		}}, nil
	}}
	e := &ECS{api: fake}

	got, err := e.ListClusters(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "prod-cluster" {
		t.Fatalf("unexpected clusters: %+v", got)
	}
}

func TestECSListClustersPaginates(t *testing.T) {
	var calls int
	fake := &fakeECS{listClusters: func(in *ecs.ListClustersInput) (*ecs.ListClustersOutput, error) {
		calls++
		if in.NextToken == nil {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{"arn:aws:ecs:r:1:cluster/a", "arn:aws:ecs:r:1:cluster/b"},
				NextToken:   aws.String("page2"),
			}, nil
		}
		return &ecs.ListClustersOutput{ClusterArns: []string{"arn:aws:ecs:r:1:cluster/c"}}, nil
	}}
	e := &ECS{api: fake}

	got, err := e.ListClusters(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 clusters across pages, got %d: %+v", len(got), got)
	}
	if calls != 2 {
		t.Errorf("expected 2 paginated calls, got %d", calls)
	}
}

func TestECSListServicesPaginates(t *testing.T) {
	var calls int
	fake := &fakeECS{listServices: func(in *ecs.ListServicesInput) (*ecs.ListServicesOutput, error) {
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
	}}
	e := &ECS{api: fake}

	got, err := e.ListServices(context.Background(), "cluster")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 12 {
		t.Fatalf("expected 12 services across pages, got %d", len(got))
	}
	if got[11].Name != "svc-11" {
		t.Errorf("unexpected last service: %q", got[11].Name)
	}
	if calls != 2 {
		t.Errorf("expected 2 paginated calls, got %d", calls)
	}
}

func TestECSGetTaskSecrets(t *testing.T) {
	fake := &fakeECS{
		describeServices: func(*ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error) {
			return &ecs.DescribeServicesOutput{Services: []ecstypes.Service{
				{TaskDefinition: aws.String("td:1")},
			}}, nil
		},
		describeTaskDef: func(*ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error) {
			return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{{
					Secrets: []ecstypes.Secret{
						{Name: aws.String("DB_PASS"), ValueFrom: aws.String("/prod/api/db_pass")},
					},
				}},
			}}, nil
		},
	}
	e := &ECS{api: fake}

	secrets, raw, err := e.GetTaskSecrets(context.Background(), "prod-cluster", "api")
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets) != 1 || secrets[0].EnvVarName != "DB_PASS" || secrets[0].SSMPath != "/prod/api/db_pass" {
		t.Fatalf("unexpected secrets: %+v", secrets)
	}
	if len(raw) == 0 {
		t.Error("expected non-empty raw JSON")
	}
}

func TestECSGetTaskSecretsServiceNotFound(t *testing.T) {
	fake := &fakeECS{describeServices: func(*ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error) {
		return &ecs.DescribeServicesOutput{}, nil
	}}
	e := &ECS{api: fake}

	if _, _, err := e.GetTaskSecrets(context.Background(), "c", "missing"); err == nil {
		t.Fatal("expected error for missing service")
	}
}
