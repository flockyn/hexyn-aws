package aws

import (
	"context"
	"hexyn-aws/internal/utils"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type mockSSM struct {
	SSMAPI
	GetParametersByPathFunc func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
	GetParametersFunc       func(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error)
	PutParameterFunc        func(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
}

func (m *mockSSM) GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	return m.GetParametersByPathFunc(ctx, params, optFns...)
}
func (m *mockSSM) GetParameters(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error) {
	return m.GetParametersFunc(ctx, params, optFns...)
}
func (m *mockSSM) PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	return m.PutParameterFunc(ctx, params, optFns...)
}

func TestGetParametersByPath(t *testing.T) {
	mock := &mockSSM{
		GetParametersByPathFunc: func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			return &ssm.GetParametersByPathOutput{
				Parameters: []ssmtypes.Parameter{{Name: aws.String("/p/r/K1"), Value: aws.String("V1"), Type: ssmtypes.ParameterTypeSecureString}},
			}, nil
		},
	}
	client := &SSMClient{client: mock}
	res, _ := client.GetParametersByPath(context.Background(), "p", "r")
	if len(res) != 1 || res[0].Name != "K1" || res[0].Type != "SecureString" {
		t.Error("failed")
	}
}

func TestGetParametersByNamesExtended(t *testing.T) {
	mock := &mockSSM{
		GetParametersFunc: func(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error) {
			return &ssm.GetParametersOutput{
				Parameters: []ssmtypes.Parameter{
					{Name: aws.String("full-arn-123"), Value: aws.String("V1")},
					{Name: aws.String("K2"), Value: aws.String("V2")},
				},
			}, nil
		},
	}
	client := &SSMClient{client: mock}
	// Test partial match and empty input
	res, _ := client.GetParametersByNames(context.Background(), map[string]string{"ENV1": "full-arn-123", "ENV2": "K2"})
	if len(res) != 2 {
		t.Errorf("expected 2, got %d", len(res))
	}

	res, _ = client.GetParametersByNames(context.Background(), nil)
	if res != nil {
		t.Error("expected nil")
	}
}

func TestPutParameters(t *testing.T) {
	mock := &mockSSM{
		PutParameterFunc: func(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			return &ssm.PutParameterOutput{}, nil
		},
	}
	client := &SSMClient{client: mock}
	success, _ := client.PutParameters(context.Background(), "p", "r", []utils.Parameter{{Name: "K", Value: "V", Type: "SecureString"}}, 1)
	if success != 1 {
		t.Error("failed")
	}
}

func TestNewSSMClientError(t *testing.T) {
	BaseDir = "non_existent"
	_, err := NewSSMClient(context.Background(), "")
	if err == nil {
		t.Error("expected error")
	}
}
