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
	"github.com/aws/smithy-go"
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

func TestSSMGetByPathFollowsPagination(t *testing.T) {
	api := mocks.NewMockSSMAPI(t)
	api.EXPECT().GetParametersByPath(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, in *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			if in.NextToken == nil {
				return &ssm.GetParametersByPathOutput{
					Parameters: []ssmtypes.Parameter{{Name: aws.String("/prod/api/A"), Value: aws.String("1")}},
					NextToken:  aws.String("page2"),
				}, nil
			}
			assert.Equal(t, "page2", aws.ToString(in.NextToken))
			return &ssm.GetParametersByPathOutput{
				Parameters: []ssmtypes.Parameter{{Name: aws.String("/prod/api/B"), Value: aws.String("2")}},
			}, nil
		})
	s := &SSM{api: api, concurrency: 2}

	got, err := s.GetByPath(context.Background(), awsx.ParamPath{Env: "prod", Repo: "api"})
	require.NoError(t, err)
	require.Len(t, got, 2, "expected params from both pages")
	assert.Equal(t, "A", got[0].Name)
	assert.Equal(t, "B", got[1].Name)
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

type mockAPIError struct {
	code string
	msg  string
}

func (m mockAPIError) ErrorCode() string             { return m.code }
func (m mockAPIError) ErrorMessage() string          { return m.msg }
func (m mockAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultUnknown }
func (m mockAPIError) Error() string                 { return fmt.Sprintf("%s: %s", m.code, m.msg) }

func TestFormatAWS(t *testing.T) {
	apiErr := mockAPIError{code: "AccessDenied", msg: "User is not authorized"}
	wrapped := fmt.Errorf("wrapper: %w", apiErr)

	err := formatAWS(wrapped)
	require.NotNil(t, err)
	assert.Equal(t, "User is not authorized", err.Error())

	stdErr := errors.New("standard error")
	assert.Equal(t, "standard error", formatAWS(stdErr).Error())

	assert.Nil(t, formatAWS(nil))
}
