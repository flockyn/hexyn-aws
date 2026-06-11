package awsx

import (
	"context"
	"errors"
	"testing"
)

// fakeLoader is a credentialLoader stub for exercising buildConfig.
type fakeLoader struct {
	loaded LoadedCredentials
	err    error
}

func (f fakeLoader) Load() (LoadedCredentials, error) { return f.loaded, f.err }

func TestBuildConfigExplicitRegionWins(t *testing.T) {
	loader := fakeLoader{loaded: LoadedCredentials{CredsMap: map[string]string{"region": "ap-southeast-3"}}}

	cfg, err := BuildConfig(context.Background(), loader, "us-east-1")
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("explicit region should win, got %q", cfg.Region)
	}
}

func TestBuildConfigFallsBackToCredsRegion(t *testing.T) {
	loader := fakeLoader{loaded: LoadedCredentials{CredsMap: map[string]string{"region": "ap-southeast-3"}}}

	cfg, err := BuildConfig(context.Background(), loader, "")
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.Region != "ap-southeast-3" {
		t.Errorf("expected fallback to creds region, got %q", cfg.Region)
	}
}

func TestBuildConfigPropagatesLoadError(t *testing.T) {
	loader := fakeLoader{err: errors.New("no creds")}

	if _, err := BuildConfig(context.Background(), loader, "us-east-1"); err == nil {
		t.Fatal("expected error when credential load fails")
	}
}
