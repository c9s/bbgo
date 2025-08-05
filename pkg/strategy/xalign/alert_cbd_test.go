package xalign

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestObjectID(t *testing.T) {
	t.Run("SameObjectID", func(t *testing.T) {
		cbd1 := &CriticalBalanceDiscrepancyAlert{
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
			Side:          types.SideTypeBuy,
		}
		cbd2 := &CriticalBalanceDiscrepancyAlert{
			BaseCurrency:  cbd1.BaseCurrency,
			QuoteCurrency: cbd1.QuoteCurrency,
			Side:          cbd1.Side,
		}
		assert.Equal(t, cbd1.ObjectID(), cbd2.ObjectID())
	})

	t.Run("DifferentObjectID", func(t *testing.T) {
		cbd1 := &CriticalBalanceDiscrepancyAlert{
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
			Side:          types.SideTypeBuy,
		}
		cbd2 := &CriticalBalanceDiscrepancyAlert{
			BaseCurrency:  "ETH",
			QuoteCurrency: "USDT",
			Side:          types.SideTypeBuy,
		}
		// different base currency
		assert.NotEqual(t, cbd1.ObjectID(), cbd2.ObjectID())
		// different side
		cbd2.BaseCurrency = cbd1.BaseCurrency
		cbd2.Side = cbd1.Side.Reverse()
		assert.NotEqual(t, cbd1.ObjectID(), cbd2.ObjectID())
		// different quote currency
		cbd2.Side = cbd1.Side
		cbd2.BaseCurrency = cbd1.BaseCurrency
		cbd2.QuoteCurrency = "USD"
		assert.NotEqual(t, cbd1.ObjectID(), cbd2.ObjectID())
	})
}
