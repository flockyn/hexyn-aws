package awsx

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type fakeEC2 struct {
	out *ec2.DescribeRegionsOutput
	err error
}

func (f fakeEC2) DescribeRegions(_ context.Context, _ *ec2.DescribeRegionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	return f.out, f.err
}

func TestSessionsListRegionsSuccess(t *testing.T) {
	fake := fakeEC2{out: &ec2.DescribeRegionsOutput{Regions: []ec2types.Region{
		{RegionName: aws.String("ap-southeast-3")},
		{RegionName: aws.String("us-east-1")},
	}}}

	got, err := (&Sessions{}).listRegions(context.Background(), fake)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "ap-southeast-3" {
		t.Fatalf("unexpected regions: %v", got)
	}
}

func TestSessionsListRegionsFallback(t *testing.T) {
	got, err := (&Sessions{}).listRegions(context.Background(), fakeEC2{err: errors.New("network down")})
	if err != nil {
		t.Fatalf("fallback should not return an error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected non-empty fallback region list")
	}
}
