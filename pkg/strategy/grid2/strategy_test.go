package grid2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestStrategy_checkRequiredInvestmentByQuantity(t *testing.T) {
	s := &Strategy{
		Market: types.Market{
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
		},
	}

	t.Run("basic base balance check", func(t *testing.T) {
		_, _, err := s.checkRequiredInvestmentByQuantity(number(2.0), number(10_000.0),
			number(1.0), number(10_000.0),
			number(0.1), number(19000.0), []Pin{})
		assert.Error(t, err)
		assert.EqualError(t, err, "baseInvestment setup 2.000000 is greater than the total base balance 1.000000")
	})

	t.Run("basic quote balance check", func(t *testing.T) {
		_, _, err := s.checkRequiredInvestmentByQuantity(number(1.0), number(10_000.0),
			number(1.0), number(100.0),
			number(0.1), number(19_000.0), []Pin{})
		assert.Error(t, err)
		assert.EqualError(t, err, "quoteInvestment setup 10000.000000 is greater than the total quote balance 100.000000")
	})

	t.Run("quote to base balance conversion check", func(t *testing.T) {
		_, requiredQuote, err := s.checkRequiredInvestmentByQuantity(number(0.0), number(10_000.0),
			number(0.0), number(10_000.0),
			number(0.1), number(13_500.0), []Pin{
				Pin(number(10_000.0)), // 0.1 * 10_000 = 1000 USD (buy)
				Pin(number(11_000.0)), // 0.1 * 11_000 = 1100 USD (buy)
				Pin(number(12_000.0)), // 0.1 * 12_000 = 1200 USD (buy)
				Pin(number(13_000.0)), // 0.1 * 13_000 = 1300 USD (buy)
				Pin(number(14_000.0)), // 0.1 * 14_000 = 1400 USD (buy)
				Pin(number(15_000.0)), // 0.1 * 15_000 = 1500 USD
			})
		assert.NoError(t, err)
		assert.Equal(t, number(6000.0), requiredQuote)
	})

	t.Run("quote to base balance conversion not enough", func(t *testing.T) {
		_, requiredQuote, err := s.checkRequiredInvestmentByQuantity(number(0.0), number(5_000.0),
			number(0.0), number(5_000.0),
			number(0.1), number(13_500.0), []Pin{
				Pin(number(10_000.0)), // 0.1 * 10_000 = 1000 USD (buy)
				Pin(number(11_000.0)), // 0.1 * 11_000 = 1100 USD (buy)
				Pin(number(12_000.0)), // 0.1 * 12_000 = 1200 USD (buy)
				Pin(number(13_000.0)), // 0.1 * 13_000 = 1300 USD (buy)
				Pin(number(14_000.0)), // 0.1 * 14_000 = 1400 USD (buy)
				Pin(number(15_000.0)), // 0.1 * 15_000 = 1500 USD
			})
		assert.EqualError(t, err, "quote balance (5000.000000 USDT) is not enough, required = quote 6000.000000")
		assert.Equal(t, number(6000.0), requiredQuote)
	})
}

func TestStrategy_checkRequiredInvestmentByAmount(t *testing.T) {
	s := &Strategy{
		Market: types.Market{
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
		},
	}

	t.Run("quote to base balance conversion", func(t *testing.T) {
		_, requiredQuote, err := s.checkRequiredInvestmentByAmount(number(0.0), number(3_000.0),
			number(0.0), number(3_000.0),
			number(1000.0),
			number(13_500.0), []Pin{
				Pin(number(10_000.0)),
				Pin(number(11_000.0)),
				Pin(number(12_000.0)),
				Pin(number(13_000.0)),
				Pin(number(14_000.0)),
				Pin(number(15_000.0)),
			})
		assert.EqualError(t, err, "quote balance (3000.000000 USDT) is not enough, required = quote 4999.999890")
		assert.Equal(t, number(4999.99989), requiredQuote)
	})
}
