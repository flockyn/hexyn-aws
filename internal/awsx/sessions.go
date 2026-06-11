package awsx

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Sessions validates the active AWS session and lists regions, using STS, IAM,
// EC2, and Organizations. (Named in the plural to avoid colliding with Session.)
type Sessions struct {
	creds CredentialLoader
}

// NewSessions creates a Sessions client backed by the given credential store.
func NewSessions(creds CredentialLoader) *Sessions {
	return &Sessions{creds: creds}
}

// Check validates the current credentials and returns the active Session.
// Returns ErrCredentialsMissing / ErrCredentialsExpired on failure.
func (s *Sessions) Check(ctx context.Context) (Session, error) {
	loaded, err := s.creds.Load()
	if err != nil {
		return Session{CredSource: loaded.Source}, err
	}

	if loaded.CredsMap["aws_access_key_id"] == "" || loaded.CredsMap["aws_secret_access_key"] == "" {
		return Session{CredSource: loaded.Source, Profile: loaded.Profile}, ErrCredentialsExpired
	}

	cfg, err := BuildConfig(ctx, s.creds, "")
	if err != nil {
		return Session{}, err
	}

	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return Session{
			CredSource: loaded.Source,
			Profile:    loaded.Profile,
			Region:     loaded.CredsMap["region"],
		}, ErrCredentialsExpired
	}

	accountID := aws.ToString(out.Account)
	return Session{
		ARN:          aws.ToString(out.Arn),
		AccountID:    accountID,
		AccountAlias: s.discoverAccountName(ctx, cfg, accountID),
		Profile:      loaded.Profile,
		Region:       loaded.CredsMap["region"],
		CredSource:   loaded.Source,
	}, nil
}

// ListEnabledRegions returns all enabled AWS regions, falling back to a common
// list if the EC2 API call fails.
func (s *Sessions) ListEnabledRegions(ctx context.Context) ([]string, error) {
	cfg, err := BuildConfig(ctx, s.creds, "")
	if err != nil {
		return nil, err
	}
	out, err := ec2.NewFromConfig(cfg).DescribeRegions(ctx, &ec2.DescribeRegionsInput{AllRegions: aws.Bool(false)})
	if err != nil {
		// No region resolved from the credentials file (or the EC2 call failed):
		// fall back to a common list so the region picker still has options.
		return []string{"ap-southeast-3", "ap-southeast-1", "us-east-1", "us-west-2"}, nil
	}
	regions := make([]string, 0, len(out.Regions))
	for _, r := range out.Regions {
		regions = append(regions, aws.ToString(r.RegionName))
	}
	return regions, nil
}

// discoverAccountName resolves a human-readable account name via IAM alias then
// AWS Organizations, returning "" when neither is available. The receiver is
// unused; it keeps the helper grouped on Sessions.
func (*Sessions) discoverAccountName(ctx context.Context, cfg aws.Config, accountID string) string {
	aliasOut, err := iam.NewFromConfig(cfg).ListAccountAliases(ctx, &iam.ListAccountAliasesInput{})
	if err == nil && len(aliasOut.AccountAliases) > 0 {
		return aliasOut.AccountAliases[0]
	}

	orgOut, err := organizations.NewFromConfig(cfg).DescribeAccount(ctx, &organizations.DescribeAccountInput{
		AccountId: &accountID,
	})
	if err == nil && orgOut.Account != nil && orgOut.Account.Name != nil {
		return aws.ToString(orgOut.Account.Name)
	}
	return ""
}
