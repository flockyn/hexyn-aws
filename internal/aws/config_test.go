package aws

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func TestSetBaseDir(t *testing.T) {
	SetBaseDir(true)
	if BaseDir != ".hexyn-aws" || !IsInitMode {
		t.Errorf("wrong local mode: %s %v", BaseDir, IsInitMode)
	}

	SetBaseDir(false)
	if IsInitMode {
		t.Error("IsInitMode should be false")
	}
}

func TestGetCredentialsPath(t *testing.T) {
	SetBaseDir(true)
	expected := filepath.Join(".hexyn-aws", "credentials")
	if GetCredentialsPath() != expected {
		t.Errorf("expected %s, got %s", expected, GetCredentialsPath())
	}
}

func TestParseCredentialsFile(t *testing.T) {
	content := `
# Comments
[profile]
aws_access_key_id=AKIA
aws_secret_access_key=SECRET
aws_session_token=TOKEN
region=ap-southeast-3
key=val
`
	tmpDir, _ := os.MkdirTemp("", "hexyn-test")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	path := filepath.Join(tmpDir, "credentials")
	_ = os.WriteFile(path, []byte(content), 0600)

	creds, profile, err := parseCredentialsFile(path)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if profile != "profile" {
		t.Errorf("expected profile, got %s", profile)
	}
	if creds["aws_access_key_id"] != "AKIA" {
		t.Error("wrong access key")
	}
}

func TestLoadAWSConfig(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "hexyn-config")
	defer func() { _ = os.RemoveAll(tmpDir) }()
	BaseDir = tmpDir

	// Case 1: Missing
	_, _, _, _, err := LoadAWSConfig(context.Background(), "")
	if err == nil || err.Error() != "MISSING" {
		t.Errorf("expected MISSING, got %v", err)
	}

	// Case 2: Expired/Empty
	path := GetCredentialsPath()
	_ = os.WriteFile(path, []byte("[default]\nregion=us-east-1"), 0600)
	_, _, _, _, err = LoadAWSConfig(context.Background(), "")
	if err == nil || err.Error() != "EXPIRED" {
		t.Errorf("expected EXPIRED, got %v", err)
	}

	// Case 3: Valid
	_ = os.WriteFile(path, []byte("[default]\naws_access_key_id=A\naws_secret_access_key=S\nregion=us-east-1"), 0600)
	_, src, reg, prof, err := LoadAWSConfig(context.Background(), "us-west-2")
	if err != nil {
		t.Fatal(err)
	}
	if src != path || reg != "us-east-1" || prof != "default" {
		t.Error("metadata mismatch")
	}
}

func TestCheckSessionError(t *testing.T) {
	BaseDir = "non_existent"
	_, _, _, _, _, _, err := CheckSession(context.Background())
	if err == nil {
		t.Error("expected error")
	}
}

func TestSaveFullCredentials(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "hexyn-save")
	defer func() { _ = os.RemoveAll(tmpDir) }()
	BaseDir = tmpDir

	err := SaveFullCredentials("A", "S", "T", "arn:aws:sts::123:assumed-role/MyRole/session")
	if err != nil {
		t.Fatal(err)
	}

	path := GetCredentialsPath()
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "[MyRole]") {
		t.Error("profile name not extracted correctly")
	}
}

func TestUpdateRegionInConfig(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "hexyn-region")
	defer func() { _ = os.RemoveAll(tmpDir) }()
	BaseDir = tmpDir

	path := GetCredentialsPath()
	_ = os.WriteFile(path, []byte("[default]\nregion=old"), 0600)

	err := UpdateRegionInConfig("new-region")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "region=new-region") {
		t.Error("region not updated")
	}

	// No region line case
	_ = os.WriteFile(path, []byte("[default]\nkey=val"), 0600)
	_ = UpdateRegionInConfig("another")
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "region=another") {
		t.Error("region not appended")
	}
}

func TestEnsureDirectories(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "hexyn-dirs")
	defer func() { _ = os.RemoveAll(tmpDir) }()
	BaseDir = tmpDir

	err := EnsureDirectories()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "output")); os.IsNotExist(err) {
		t.Error("output dir missing")
	}
}

func TestAccountLabels(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "hexyn-labels")
	defer func() { _ = os.RemoveAll(tmpDir) }()
	BaseDir = tmpDir

	err := SaveLabel("123", "MyAccount")
	if err != nil {
		t.Fatal(err)
	}

	labels := LoadLabels()
	if labels["123"] != "MyAccount" {
		t.Error("label not saved/loaded")
	}
}

func TestDiscoverAccountNameLocal(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "hexyn-disc")
	defer func() { _ = os.RemoveAll(tmpDir) }()
	BaseDir = tmpDir

	_ = SaveLabel("999", "LocalName")

	name := discoverAccountName(context.Background(), aws.Config{}, "999")
	if name != "LocalName" {
		t.Errorf("expected LocalName, got %s", name)
	}
}

type mockEC2 struct {
	EC2API
}

func (m *mockEC2) DescribeRegions(ctx context.Context, params *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	return &ec2.DescribeRegionsOutput{
		Regions: []types.Region{{RegionName: aws.String("r1")}},
	}, nil
}

func TestListEnabledRegionsWithClient(t *testing.T) {
	regions, _ := listEnabledRegionsWithClient(context.Background(), &mockEC2{})
	if len(regions) != 1 || regions[0] != "r1" {
		t.Error("failed to list regions")
	}
}

type mockEC2Error struct {
	EC2API
}

func (m *mockEC2Error) DescribeRegions(ctx context.Context, params *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	return nil, fmt.Errorf("error")
}

func TestListEnabledRegionsWithClientError(t *testing.T) {
	regions, _ := listEnabledRegionsWithClient(context.Background(), &mockEC2Error{})
	if len(regions) < 4 {
		t.Error("fallback failed")
	}
}

type mockSTS struct {
	STSAPI
}

func (m *mockSTS) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn:     aws.String("arn:123"),
		Account: aws.String("123"),
	}, nil
}

func TestCheckSessionWithClient(t *testing.T) {
	arn, id, _, _, _, _, _ := checkSessionWithClient(context.Background(), aws.Config{}, &mockSTS{}, "src", "reg", "prof")
	if arn != "arn:123" || id != "123" {
		t.Error("failed to check session")
	}
}
