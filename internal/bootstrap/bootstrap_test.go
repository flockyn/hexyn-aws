package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"hexyn-aws/internal/config"
)

func TestNewServiceReturnsService(t *testing.T) {
	assert.NotNil(t, NewService(config.New(true)), "NewService returned nil")
}
