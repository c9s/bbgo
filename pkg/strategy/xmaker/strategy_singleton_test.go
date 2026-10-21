package xmaker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func newValidatableStrategy(symbol string) *Strategy {
	s := &Strategy{
		Symbol: symbol,
	}
	s.MakerSession = "maker"
	s.HedgeSession = "hedge"
	s.Quantity = fixedpoint.NewFromFloat(1.0)
	return s
}

func TestStrategy_Validate_SingletonPerSymbol(t *testing.T) {
	t.Run("two instances with the same symbol: second one fails", func(t *testing.T) {
		instanceRegistryMu.Lock()
		instanceRegistry = make(map[string]*Strategy)
		instanceRegistryMu.Unlock()

		s1 := newValidatableStrategy("BTCUSDT")
		s2 := newValidatableStrategy("BTCUSDT")

		assert.NoError(t, s1.Validate())

		err := s2.Validate()
		assert.Error(t, err)
	})

	t.Run("two instances with different symbols: both succeed", func(t *testing.T) {
		instanceRegistryMu.Lock()
		instanceRegistry = make(map[string]*Strategy)
		instanceRegistryMu.Unlock()

		s1 := newValidatableStrategy("BTCUSDT")
		s2 := newValidatableStrategy("ETHUSDT")

		assert.NoError(t, s1.Validate())
		assert.NoError(t, s2.Validate())
	})

	t.Run("re-validating the same instance is idempotent", func(t *testing.T) {
		instanceRegistryMu.Lock()
		instanceRegistry = make(map[string]*Strategy)
		instanceRegistryMu.Unlock()

		s1 := newValidatableStrategy("BTCUSDT")

		assert.NoError(t, s1.Validate())
		assert.NoError(t, s1.Validate())
	})
}
