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

	"hexyn-aws/internal/awsx"
)

type fakeSSM struct {
	byPath   func(*ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)
	byNames  func(*ssm.GetParametersInput) (*ssm.GetParametersOutput, error)
	putCalls int32
	putErrOn string // parameter name that should fail
}

func (f *fakeSSM) GetParametersByPath(_ context.Context, in *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	return f.byPath(in)
}

func (f *fakeSSM) GetParameters(_ context.Context, in *ssm.GetParametersInput, _ ...func(*ssm.Options)) (*ssm.GetParametersOutput, error) {
	return f.byNames(in)
}

func (f *fakeSSM) PutParameter(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	atomic.AddInt32(&f.putCalls, 1)
	if aws.ToString(in.Name) == f.putErrOn {
		return nil, errors.New("boom")
	}
	return &ssm.PutParameterOutput{}, nil
}

func TestSSMGetByPathTrimsPrefix(t *testing.T) {
	fake := &fakeSSM{byPath: func(in *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error) {
		if got := aws.ToString(in.Path); got != "/prod/api/" {
			t.Errorf("unexpected path %q", got)
		}
		return &ssm.GetParametersByPathOutput{Parameters: []ssmtypes.Parameter{
			{Name: aws.String("/prod/api/DB_HOST"), Value: aws.String("localhost"), Type: ssmtypes.ParameterTypeString},
			{Name: aws.String("/prod/api/DB_PASS"), Value: aws.String("s3cret"), Type: ssmtypes.ParameterTypeSecureString},
		}}, nil
	}}
	s := &SSM{api: fake, concurrency: 2}

	got, err := s.GetByPath(context.Background(), awsx.ParamPath{Env: "prod", Repo: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "DB_HOST" {
		t.Fatalf("unexpected params: %+v", got)
	}
	if !got[1].IsSecure() {
		t.Error("DB_PASS should be SecureString")
	}
}

func TestSSMGetByNamesBatchesByTen(t *testing.T) {
	var batches int
	fake := &fakeSSM{byNames: func(in *ssm.GetParametersInput) (*ssm.GetParametersOutput, error) {
		batches++
		if len(in.Names) > 10 {
			t.Errorf("batch too large: %d", len(in.Names))
		}
		out := &ssm.GetParametersOutput{}
		for _, n := range in.Names {
			out.Parameters = append(out.Parameters, ssmtypes.Parameter{Name: aws.String(n), Value: aws.String("v")})
		}
		return out, nil
	}}
	s := &SSM{api: fake, concurrency: 2}

	nameMap := map[string]string{}
	for i := range 23 {
		nameMap[fmt.Sprintf("VAR_%d", i)] = fmt.Sprintf("/path/%d", i)
	}
	got, err := s.GetByNames(context.Background(), nameMap)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 23 {
		t.Errorf("expected 23 resolved params, got %d", len(got))
	}
	if batches != 3 { // 10 + 10 + 3
		t.Errorf("expected 3 batches, got %d", batches)
	}
}

func TestSSMPutCountsSuccessesAndErrors(t *testing.T) {
	fake := &fakeSSM{putErrOn: "/prod/api/BAD"}
	s := &SSM{api: fake, concurrency: 3}

	params := []awsx.Parameter{
		{Name: "GOOD1", Value: "a"},
		{Name: "GOOD2", Value: "b"},
		{Name: "BAD", Value: "c"},
	}
	ok, errs := s.Put(context.Background(), awsx.ParamPath{Env: "prod", Repo: "api"}, params)
	if ok != 2 {
		t.Errorf("expected 2 successes, got %d", ok)
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
	if fake.putCalls != 3 {
		t.Errorf("expected 3 PutParameter calls, got %d", fake.putCalls)
	}
}
