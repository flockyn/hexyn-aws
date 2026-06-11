package bootstrap

import (
	"testing"

	"hexyn-aws/internal/config"
)

func TestNewServiceReturnsService(t *testing.T) {
	if NewService(config.New(true)) == nil {
		t.Fatal("NewService returned nil")
	}
}
