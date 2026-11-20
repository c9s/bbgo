package bbgo

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/exchange"
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

func TestEnvironment_SessionNamesMustBeIdentifiers(t *testing.T) {
	t.Run("a valid name is added", func(t *testing.T) {
		environ := NewEnvironment()

		err := environ.AddExchangesFromSessionConfig(map[string]*ExchangeSession{
			"binance_margin_2": publicSession(),
		})

		require.NoError(t, err)
		session, ok := environ.Session("binance_margin_2")
		require.True(t, ok)
		assert.Equal(t, "binance_margin_2", session.Name)
	})

	for name, bad := range map[string]string{
		"uppercase": "Binance",
		"hyphen":    "binance-margin",
		"space":     "binance margin",
		"empty":     "",
		"too long":  strings.Repeat("a", types.MaxIdentifierLength+1),
	} {
		t.Run(name+" is rejected", func(t *testing.T) {
			environ := NewEnvironment()

			err := environ.AddExchangesFromSessionConfig(map[string]*ExchangeSession{bad: publicSession()})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid session name")
		})
	}

	t.Run("no session is created when one name is bad, and every bad name is reported", func(t *testing.T) {
		environ := NewEnvironment()

		err := environ.ConfigureExchangeSessions(&Config{Sessions: map[string]*ExchangeSession{
			"binance":        publicSession(),
			"Binance_Margin": publicSession(),
			"max-spot":       publicSession(),
		}})

		require.Error(t, err)
		assert.Contains(t, err.Error(), `"Binance_Margin"`)
		assert.Contains(t, err.Error(), `"max-spot"`)
		assert.NotContains(t, err.Error(), `"binance"`)
		assert.Empty(t, environ.Sessions())
	})
}

// A config cannot define a session under a name the environment already has
func TestEnvironment_SessionNamesCannotBeDefinedTwice(t *testing.T) {
	environ := NewEnvironment()
	require.NoError(t, environ.AddExchangesFromSessionConfig(map[string]*ExchangeSession{
		"binance": publicSession(),
	}))
	first, _ := environ.Session("binance")

	err := environ.AddExchangesFromSessionConfig(map[string]*ExchangeSession{
		"binance":     publicSession(),
		"binance_two": publicSession(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `session "binance" is already defined`)

	// the first session is kept, and nothing of the rejected config is added
	kept, _ := environ.Session("binance")
	assert.Same(t, first, kept)
	assert.Len(t, environ.Sessions(), 1)
}

// Within one environment a session name refers to exactly one session
func TestEnvironment_AddingASessionUnderATakenNamePanics(t *testing.T) {
	environ := NewEnvironment()
	first := environ.AddExchangeSession("binance", publicSession())

	assert.PanicsWithError(t, `session "binance" is already defined`, func() {
		environ.AddExchangeSession("binance", publicSession())
	})
	ex, err := exchange.NewPublic(types.ExchangeBinance)
	require.NoError(t, err)
	assert.PanicsWithError(t, `session "binance" is already defined`, func() {
		environ.AddExchange("binance", ex)
	})

	// the first session is kept
	kept, _ := environ.Session("binance")
	assert.Same(t, first, kept)
	assert.Len(t, environ.Sessions(), 1)

	// another name is fine
	environ.AddExchangeSession("binance_margin", publicSession())
	assert.Len(t, environ.Sessions(), 2)
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

	// a bad name in the config does not stop a backtest: the sessions are not read
	require.NoError(t, backtest.ConfigureExchangeSessions(&Config{
		Sessions: map[string]*ExchangeSession{"Binance-Margin": publicSession()},
	}))

	// the simulated session takes the live name, and nothing is replaced
	assert.NotPanics(t, func() {
		backtest.AddExchangeSession("binance", publicSession())
	})
}
