package awsx

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// BuildConfig constructs an aws.Config from the stored credentials, scoped to
// region (falling back to the credentials file's region when empty). It is
// exported so the awsx/service clients can build their region-scoped configs.
func BuildConfig(ctx context.Context, creds CredentialLoader, region string) (aws.Config, error) {
	loaded, err := creds.Load()
	if err != nil {
		return aws.Config{}, fmt.Errorf("unable to load credentials: %w", err)
	}

	opts := []func(*config.LoadOptions) error{
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     loaded.CredsMap["aws_access_key_id"],
				SecretAccessKey: loaded.CredsMap["aws_secret_access_key"],
				SessionToken:    loaded.CredsMap["aws_session_token"],
			},
		}),
	}
	switch {
	case region != "":
		opts = append(opts, config.WithRegion(region))
	case loaded.CredsMap["region"] != "":
		opts = append(opts, config.WithRegion(loaded.CredsMap["region"]))
	}
	return config.LoadDefaultConfig(ctx, opts...)
}
