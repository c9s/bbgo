package bbgo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/types"
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

func publicSession() *ExchangeSession {
	return &ExchangeSession{ExchangeName: types.ExchangeBinance, PublicOnly: true}
}

// A backtest adds a simulated session under the name of the live session it
// stands in for, so configuring the sessions must not create the live ones.
func TestEnvironment_ConfigureExchangeSessions_CreatesNoneInABacktest(t *testing.T) {
	config := func() *Config {
		return &Config{Sessions: map[string]*ExchangeSession{"binance": publicSession()}}
	}

	live := NewEnvironment()
	require.NoError(t, BootstrapEnvironmentLightweight(context.Background(), live, config()))
	assert.Len(t, live.Sessions(), 1)

	IsBackTesting = true
	t.Cleanup(func() { IsBackTesting = false })

	backtest := NewEnvironment()
	require.NoError(t, BootstrapEnvironmentLightweight(context.Background(), backtest, config()))
	assert.Empty(t, backtest.Sessions())

	// the simulated session is the only one under the live name
	simulated := backtest.AddExchangeSession("binance", publicSession())
	session, ok := backtest.Session("binance")
	require.True(t, ok)
	assert.Same(t, simulated, session)
	assert.Len(t, backtest.Sessions(), 1)
}
