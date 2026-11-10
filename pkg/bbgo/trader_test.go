package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrader_SetupRPCRegistry(t *testing.T) {
	environ := NewEnvironment()
	trader := NewTrader(environ)

	assert.Nil(t, trader.RPCRegistry())

	trader.SetupRPCRegistry()

	reg := trader.RPCRegistry()
	require.NotNil(t, reg)

	methods := reg.Methods()
	require.Len(t, methods, 1)
	assert.Equal(t, "ping", methods[0].Name)
	assert.False(t, methods[0].Mutating)
}
