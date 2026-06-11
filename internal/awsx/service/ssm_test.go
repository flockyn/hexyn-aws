package service

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"hexyn-aws/internal/awsx"
	mocks "hexyn-aws/test/mocks/service"
)

func TestSSMGetByPathTrimsPrefix(t *testing.T) {
	api := mocks.NewMockSSMAPI(t)
	api.EXPECT().GetParametersByPath(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, in *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			assert.Equal(t, "/prod/api/", aws.ToString(in.Path))
			return &ssm.GetParametersByPathOutput{Parameters: []ssmtypes.Parameter{
				{Name: aws.String("/prod/api/DB_HOST"), Value: aws.String("localhost"), Type: ssmtypes.ParameterTypeString},
				{Name: aws.String("/prod/api/DB_PASS"), Value: aws.String("s3cret"), Type: ssmtypes.ParameterTypeSecureString},
			}}, nil
		})
	s := &SSM{api: api, concurrency: 2}

	got, err := s.GetByPath(context.Background(), awsx.ParamPath{Env: "prod", Repo: "api"})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "DB_HOST", got[0].Name)
	assert.True(t, got[1].IsSecure(), "DB_PASS should be SecureString")
}

func TestSSMGetByNamesBatchesByTen(t *testing.T) {
	var batches int32
	api := mocks.NewMockSSMAPI(t)
	api.EXPECT().GetParameters(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, in *ssm.GetParametersInput, _ ...func(*ssm.Options)) (*ssm.GetParametersOutput, error) {
			atomic.AddInt32(&batches, 1)
			assert.LessOrEqual(t, len(in.Names), 10, "batch too large")
			out := &ssm.GetParametersOutput{}
			for _, n := range in.Names {
				out.Parameters = append(out.Parameters, ssmtypes.Parameter{Name: aws.String(n), Value: aws.String("v")})
			}
			return out, nil
		})
	s := &SSM{api: api, concurrency: 2}

	nameMap := map[string]string{}
	for i := range 23 {
		nameMap[fmt.Sprintf("VAR_%d", i)] = fmt.Sprintf("/path/%d", i)
	}
	got, err := s.GetByNames(context.Background(), nameMap)
	require.NoError(t, err)
	assert.Len(t, got, 23, "expected 23 resolved params")
	assert.Equal(t, int32(3), batches, "expected 3 batches (10 + 10 + 3)")
}

func TestSSMPutCountsSuccessesAndErrors(t *testing.T) {
	var putCalls int32
	api := mocks.NewMockSSMAPI(t)
	api.EXPECT().PutParameter(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			atomic.AddInt32(&putCalls, 1)
			if aws.ToString(in.Name) == "/prod/api/BAD" {
				return nil, errors.New("boom")
			}
			return &ssm.PutParameterOutput{}, nil
		})
	s := &SSM{api: api, concurrency: 3}

	params := []awsx.Parameter{
		{Name: "GOOD1", Value: "a"},
		{Name: "GOOD2", Value: "b"},
		{Name: "BAD", Value: "c"},
	}
	ok, errs := s.Put(context.Background(), awsx.ParamPath{Env: "prod", Repo: "api"}, params)
	assert.Equal(t, 2, ok, "expected 2 successes")
	assert.Len(t, errs, 1, "expected 1 error")
	assert.Equal(t, int32(3), putCalls, "expected 3 PutParameter calls")
}
