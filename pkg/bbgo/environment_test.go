package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironment_RPCRegistry(t *testing.T) {
	environ := NewEnvironment()

	reg := environ.RPCRegistry()
	require.NotNil(t, reg, "RPCRegistry must be non-nil immediately after NewEnvironment")

	require.NoError(t, reg.Export("ping", "demo", false, func() (string, error) {
		return "pong", nil
	}))

	methods := reg.Methods()
	require.Len(t, methods, 1)
	assert.Equal(t, "ping", methods[0].Name)

	err := reg.Export("ping", "a different implementation", true, func() error { return nil })
	require.Error(t, err)
}
