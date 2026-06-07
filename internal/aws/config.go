package aws

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

var BaseDir string
var IsInitMode bool

const CredentialsPath = "credentials"
const LabelsPath = "account_labels.json"

type EC2API interface {
	DescribeRegions(ctx context.Context, params *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
}

type STSAPI interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

func SetBaseDir(local bool) {
	IsInitMode = local
	if local {
		BaseDir = ".hexyn-aws"
	} else {
		if _, err := os.Stat(".hexyn-aws"); err == nil {
			BaseDir = ".hexyn-aws"
		} else {
			home, _ := os.UserHomeDir()
			BaseDir = filepath.Join(home, ".hexyn-aws")
		}
	}
}

func EnsureDirectories() error {
	if BaseDir == "" {
		return fmt.Errorf("BaseDir not set")
	}
	err := os.MkdirAll(filepath.Join(BaseDir, "input"), 0755)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(BaseDir, "output"), 0755)
	if err != nil {
		return err
	}

	gitignorePath := filepath.Join(BaseDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		content := "# Ignore everything in this directory\n*\n!.gitignore\n"
		_ = os.WriteFile(gitignorePath, []byte(content), 0644)
	}

	return nil
}

func GetCredentialsPath() string {
	return filepath.Join(BaseDir, CredentialsPath)
}

func LoadAWSConfig(ctx context.Context, regionOverride string) (aws.Config, string, string, string, error) {
	var opts []func(*config.LoadOptions) error

	credPath := GetCredentialsPath()
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		return aws.Config{}, credPath, "", "", fmt.Errorf("MISSING")
	}

	credsMap, profile, err := parseCredentialsFile(credPath)
	if err != nil {
		return aws.Config{}, credPath, "", "", err
	}

	if credsMap["aws_access_key_id"] == "" || credsMap["aws_secret_access_key"] == "" {
		return aws.Config{}, credPath, "", profile, fmt.Errorf("EXPIRED")
	}

	opts = append(opts, config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     credsMap["aws_access_key_id"],
			SecretAccessKey: credsMap["aws_secret_access_key"],
			SessionToken:    credsMap["aws_session_token"],
		},
	}))

	configRegion := credsMap["region"]
	if regionOverride != "" {
		opts = append(opts, config.WithRegion(regionOverride))
	} else if configRegion != "" {
		opts = append(opts, config.WithRegion(configRegion))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	return cfg, credPath, configRegion, profile, err
}

func parseCredentialsFile(path string) (map[string]string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = file.Close() }()

	creds := make(map[string]string)
	profile := "default"
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			profile = strings.Trim(line, "[]")
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			creds[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, "", fmt.Errorf("error reading credentials file: %w", err)
	}

	return creds, profile, nil
}

func CheckSession(ctx context.Context) (string, string, string, string, string, string, error) {
	cfg, source, region, profile, err := LoadAWSConfig(ctx, "")
	if err != nil {
		return "", "", "", profile, source, region, err
	}
	return checkSessionWithClient(ctx, cfg, sts.NewFromConfig(cfg), source, region, profile)
}

func checkSessionWithClient(ctx context.Context, cfg aws.Config, client STSAPI, source, region, profile string) (string, string, string, string, string, string, error) {
	out, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", "", "", profile, source, region, fmt.Errorf("EXPIRED")
	}

	accountID := *out.Account
	accountAlias := discoverAccountName(ctx, cfg, accountID)

	return *out.Arn, accountID, accountAlias, profile, source, region, nil
}

func discoverAccountName(ctx context.Context, cfg aws.Config, accountID string) string {
	labels := LoadLabels()
	if label, ok := labels[accountID]; ok {
		return label
	}

	iamClient := iam.NewFromConfig(cfg)
	aliasOut, err := iamClient.ListAccountAliases(ctx, &iam.ListAccountAliasesInput{})
	if err == nil && len(aliasOut.AccountAliases) > 0 {
		return aliasOut.AccountAliases[0]
	}

	orgClient := organizations.NewFromConfig(cfg)
	orgOut, err := orgClient.DescribeAccount(ctx, &organizations.DescribeAccountInput{
		AccountId: &accountID,
	})
	if err == nil && orgOut.Account != nil && orgOut.Account.Name != nil {
		return *orgOut.Account.Name
	}
	return ""
}

func LoadLabels() map[string]string {
	labels := make(map[string]string)
	data, err := os.ReadFile(filepath.Join(BaseDir, LabelsPath))
	if err == nil {
		_ = json.Unmarshal(data, &labels)
	}
	return labels
}

func SaveLabel(accountID, label string) error {
	labels := LoadLabels()
	labels[accountID] = label
	data, _ := json.MarshalIndent(labels, "", "  ")
	return os.WriteFile(filepath.Join(BaseDir, LabelsPath), data, 0600)
}

func ListEnabledRegions(ctx context.Context) ([]string, error) {
	cfg, _, _, _, err := LoadAWSConfig(ctx, "us-east-1")
	if err != nil {
		return nil, err
	}
	return listEnabledRegionsWithClient(ctx, ec2.NewFromConfig(cfg))
}

func listEnabledRegionsWithClient(ctx context.Context, client EC2API) ([]string, error) {
	out, err := client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(false),
	})
	if err != nil {
		return []string{"ap-southeast-3", "ap-southeast-1", "us-east-1", "us-west-2"}, nil
	}

	var regions []string
	for _, r := range out.Regions {
		regions = append(regions, *r.RegionName)
	}
	return regions, nil
}

func UpdateRegionInConfig(region string) error {
	credPath := GetCredentialsPath()
	data, err := os.ReadFile(credPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "region=") {
			lines[i] = "region=" + region
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, "region="+region)
	}

	return os.WriteFile(credPath, []byte(strings.Join(lines, "\n")), 0600)
}

func SaveFullCredentials(accessKey, secretKey, token, arn string) error {
	profileName := "default"
	if strings.Contains(arn, "assumed-role") {
		parts := strings.Split(arn, "/")
		if len(parts) > 1 {
			profileName = parts[1]
		}
	}
	content := fmt.Sprintf("[%s]\naws_access_key_id=%s\naws_secret_access_key=%s\naws_session_token=%s\n", profileName, accessKey, secretKey, token)
	credPath := GetCredentialsPath()
	_ = os.MkdirAll(filepath.Dir(credPath), 0755)
	return os.WriteFile(credPath, []byte(content), 0600)
}
