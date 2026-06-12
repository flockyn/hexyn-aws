// Package service holds the region-scoped AWS service clients (SSM, ECS) and the
// Clients adapter that satisfies the secrets.AWS port. It is the one place that
// bridges the awsx infrastructure to the secrets application ports.
package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"hexyn-aws/internal/awsx"
)

const defaultPutConcurrency = 5

// SSMAPI is the subset of the AWS SSM client used here (enables test mocking).
type SSMAPI interface {
	GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
	GetParameters(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error)
	PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
}

// SSM is a region-scoped client over AWS SSM Parameter Store.
type SSM struct {
	api         SSMAPI
	concurrency int
}

// NewSSM builds an SSM client configured for the given region.
func NewSSM(ctx context.Context, creds awsx.CredentialLoader, region string) (*SSM, error) {
	cfg, err := awsx.BuildConfig(ctx, creds, region)
	if err != nil {
		return nil, fmt.Errorf("unable to build AWS config for region %s: %w", region, err)
	}
	return &SSM{api: ssm.NewFromConfig(cfg), concurrency: defaultPutConcurrency}, nil
}

// GetByPath retrieves all parameters under /<Env>/<Repo>/ from SSM, following
// pagination so every page is collected (SSM returns at most 10 per call).
func (s *SSM) GetByPath(ctx context.Context, dest awsx.ParamPath) ([]awsx.Parameter, error) {
	prefix := fmt.Sprintf("/%s/%s/", dest.Env, dest.Repo)

	var params []awsx.Parameter
	var nextToken *string
	for {
		out, err := s.api.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
			Path:           aws.String(prefix),
			Recursive:      aws.Bool(true),
			WithDecryption: aws.Bool(true),
			NextToken:      nextToken,
		})
		if err != nil {
			return nil, err
		}
		for _, p := range out.Parameters {
			params = append(params, awsx.Parameter{
				Name:  strings.TrimPrefix(aws.ToString(p.Name), prefix),
				Value: aws.ToString(p.Value),
				Type:  s.toParamType(p.Type),
			})
		}
		if out.NextToken == nil {
			return params, nil
		}
		nextToken = out.NextToken
	}
}

// GetByNames resolves SSM paths to parameters, batching in groups of 10 as the
// AWS API requires. nameMap maps env-var name → SSM path.
func (s *SSM) GetByNames(ctx context.Context, nameMap map[string]string) ([]awsx.Parameter, error) {
	ssmToEnv := make(map[string][]string)
	uniquePaths := make([]string, 0, len(nameMap))
	for envVar, ssmPath := range nameMap {
		if _, seen := ssmToEnv[ssmPath]; !seen {
			uniquePaths = append(uniquePaths, ssmPath)
		}
		ssmToEnv[ssmPath] = append(ssmToEnv[ssmPath], envVar)
	}
	if len(uniquePaths) == 0 {
		return nil, nil
	}

	var results []awsx.Parameter
	for i := 0; i < len(uniquePaths); i += 10 {
		end := min(i+10, len(uniquePaths))
		out, err := s.api.GetParameters(ctx, &ssm.GetParametersInput{
			Names:          uniquePaths[i:end],
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			return nil, err
		}
		results = append(results, s.mapParameters(out.Parameters, ssmToEnv)...)
	}
	return results, nil
}

// mapParameters fans each returned SSM parameter out to the env-var names that
// requested it, with a suffix-match fallback for short-form paths.
func (s *SSM) mapParameters(out []ssmtypes.Parameter, ssmToEnv map[string][]string) []awsx.Parameter {
	var results []awsx.Parameter
	for _, p := range out {
		name := aws.ToString(p.Name)
		envVars := ssmToEnv[name]
		if len(envVars) == 0 {
			for ssmName, eList := range ssmToEnv {
				if strings.HasSuffix(ssmName, name) || strings.HasSuffix(name, ssmName) {
					envVars = eList
					break
				}
			}
		}
		for _, ev := range envVars {
			results = append(results, awsx.Parameter{Name: ev, Value: aws.ToString(p.Value), Type: s.toParamType(p.Type)})
		}
	}
	return results
}

// Put uploads parameters to /<Env>/<Repo>/ concurrently, returning the success
// count and any per-parameter errors.
func (s *SSM) Put(ctx context.Context, dest awsx.ParamPath, params []awsx.Parameter) (int, []error) {
	concurrency := s.concurrency
	if concurrency <= 0 {
		concurrency = defaultPutConcurrency
	}

	paramChan := make(chan awsx.Parameter, len(params))
	errChan := make(chan error, len(params))
	var (
		wg           sync.WaitGroup
		mu           sync.Mutex
		successCount int
	)

	for range concurrency {
		wg.Go(func() {
			for p := range paramChan {
				if err := s.putOne(ctx, dest, p); err != nil {
					errChan <- err
				} else {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}
		})
	}

	for _, p := range params {
		paramChan <- p
	}
	close(paramChan)
	wg.Wait()
	close(errChan)

	var errs []error
	for e := range errChan {
		errs = append(errs, e)
	}
	return successCount, errs
}

func (s *SSM) putOne(ctx context.Context, dest awsx.ParamPath, p awsx.Parameter) error {
	name := fmt.Sprintf("/%s/%s/%s", dest.Env, dest.Repo, p.Name)
	paramType := ssmtypes.ParameterTypeString
	if p.Type == awsx.ParameterTypeSecureString {
		paramType = ssmtypes.ParameterTypeSecureString
	}
	_, err := s.api.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(name),
		Value:     aws.String(p.Value),
		Type:      paramType,
		Overwrite: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("failed to upload %s: %w", name, err)
	}
	return nil
}

// toParamType maps an SSM SDK parameter type to the domain type. The receiver is
// unused; it is a method to keep SSM's helpers grouped on the type.
func (*SSM) toParamType(t ssmtypes.ParameterType) awsx.ParameterType {
	if t == ssmtypes.ParameterTypeSecureString {
		return awsx.ParameterTypeSecureString
	}
	return awsx.ParameterTypeString
}
