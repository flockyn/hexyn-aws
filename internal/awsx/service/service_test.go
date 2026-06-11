package service

import (
	"context"
	"os"
	"testing"

	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/config"
)

func TestClientsErrorWithoutCredentials(t *testing.T) {
	orig, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil { // no .hexyn-aws/credentials here
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	c := NewClients(awsx.NewCredentialStore(config.New(true)))
	if _, err := c.SSM(context.Background(), "us-east-1"); err == nil {
		t.Error("expected SSM to error when credentials are missing")
	}
	if _, err := c.ECS(context.Background(), "us-east-1"); err == nil {
		t.Error("expected ECS to error when credentials are missing")
	}
}
