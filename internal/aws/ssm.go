package aws

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"hexyn-aws/internal/utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type SSMAPI interface {
	GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
	GetParameters(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error)
	PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
}

type SSMClient struct {
	client SSMAPI
}

func NewSSMClient(ctx context.Context, region string) (*SSMClient, error) {
	cfg, _, _, _, err := LoadAWSConfig(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	return &SSMClient{
		client: ssm.NewFromConfig(cfg),
	}, nil
}

func (s *SSMClient) GetParametersByPath(ctx context.Context, environment, repoName string) ([]utils.Parameter, error) {
	pathPrefix := fmt.Sprintf("/%s/%s/", environment, repoName)

	out, err := s.client.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
		Path:           aws.String(pathPrefix),
		Recursive:      aws.Bool(true),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}

	var parameters []utils.Parameter
	for _, param := range out.Parameters {
		keyName := strings.TrimPrefix(*param.Name, pathPrefix)
		paramType := "String"
		if param.Type == ssmtypes.ParameterTypeSecureString {
			paramType = "SecureString"
		}
		parameters = append(parameters, utils.Parameter{
			Name:  keyName,
			Value: *param.Value,
			Type:  paramType,
		})
	}

	return parameters, nil
}

func (s *SSMClient) GetParametersByNames(ctx context.Context, nameMap map[string]string) ([]utils.Parameter, error) {
	ssmToEnvVars := make(map[string][]string)
	var uniqueSSMNames []string

	for envVar, ssmName := range nameMap {
		if _, ok := ssmToEnvVars[ssmName]; !ok {
			uniqueSSMNames = append(uniqueSSMNames, ssmName)
		}
		ssmToEnvVars[ssmName] = append(ssmToEnvVars[ssmName], envVar)
	}

	if len(uniqueSSMNames) == 0 {
		return nil, nil
	}

	var parameters []utils.Parameter
	for i := 0; i < len(uniqueSSMNames); i += 10 {
		end := i + 10
		if end > len(uniqueSSMNames) {
			end = len(uniqueSSMNames)
		}

		out, err := s.client.GetParameters(ctx, &ssm.GetParametersInput{
			Names:          uniqueSSMNames[i:end],
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			return nil, err
		}

		for _, param := range out.Parameters {
			envVars := ssmToEnvVars[*param.Name]
			if len(envVars) == 0 {
				for ssmN, eList := range ssmToEnvVars {
					if strings.HasSuffix(ssmN, *param.Name) || strings.HasSuffix(*param.Name, ssmN) {
						envVars = eList
						break
					}
				}
			}

			paramType := "String"
			if param.Type == ssmtypes.ParameterTypeSecureString {
				paramType = "SecureString"
			}

			for _, ev := range envVars {
				parameters = append(parameters, utils.Parameter{
					Name:  ev,
					Value: *param.Value,
					Type:  paramType,
				})
			}
		}
	}

	return parameters, nil
}

func (s *SSMClient) PutParameters(ctx context.Context, environment, repoName string, params []utils.Parameter, concurrency int) (int, []error) {
	if concurrency <= 0 {
		concurrency = 5
	}

	var wg sync.WaitGroup
	paramChan := make(chan utils.Parameter, len(params))
	errChan := make(chan error, len(params))
	var successCount int
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range paramChan {
				paramName := fmt.Sprintf("/%s/%s/%s", environment, repoName, p.Name)
				paramType := ssmtypes.ParameterTypeString
				if p.Type == "SecureString" {
					paramType = ssmtypes.ParameterTypeSecureString
				}

				_, err := s.client.PutParameter(ctx, &ssm.PutParameterInput{
					Name:      aws.String(paramName),
					Value:     aws.String(p.Value),
					Type:      paramType,
					Overwrite: aws.Bool(true),
				})

				if err != nil {
					errChan <- fmt.Errorf("failed to upload %s: %w", paramName, err)
				} else {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}
		}()
	}

	for _, p := range params {
		paramChan <- p
	}
	close(paramChan)

	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	return successCount, errors
}
