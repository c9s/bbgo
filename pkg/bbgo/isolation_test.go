package bbgo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetIsolationFromContext(t *testing.T) {
	ctx := context.Background()
	isolation := GetIsolationFromContext(ctx)
	assert.NotNil(t, isolation)
	assert.NotNil(t, isolation.persistenceServiceFacade)
	assert.NotNil(t, isolation.gracefulShutdown)
}

func TestNewDefaultIsolation(t *testing.T) {
	isolation := NewDefaultIsolation()
	assert.NotNil(t, isolation)
	assert.NotNil(t, isolation.persistenceServiceFacade)
	assert.NotNil(t, isolation.gracefulShutdown)
	assert.Equal(t, persistenceServiceFacade, isolation.persistenceServiceFacade)
}
