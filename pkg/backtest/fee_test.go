package backtest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_feeModeFunctionToken(t *testing.T) {
	market := getTestMarket()
	t.Run("sellOrder", func(t *testing.T) {
		order := types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:      market.Symbol,
				Side:        types.SideTypeSell,
				Type:        types.OrderTypeLimit,
				Quantity:    fixedpoint.NewFromFloat(0.1),
				Price:       fixedpoint.NewFromFloat(20000.0),
				TimeInForce: types.TimeInForceGTC,
			},
		}
		feeRate := fixedpoint.MustNewFromString("0.075%")
		fee, feeCurrency := feeModeFunctionToken(&order, &market, feeRate)
		assert.Equal(t, "1.5", fee.String())
		assert.Equal(t, "FEE", feeCurrency)
	})

	t.Run("buyOrder", func(t *testing.T) {
		order := types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:      market.Symbol,
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimit,
				Quantity:    fixedpoint.NewFromFloat(0.1),
				Price:       fixedpoint.NewFromFloat(20000.0),
				TimeInForce: types.TimeInForceGTC,
			},
		}

		feeRate := fixedpoint.MustNewFromString("0.075%")
		fee, feeCurrency := feeModeFunctionToken(&order, &market, feeRate)
		assert.Equal(t, "1.5", fee.String())
		assert.Equal(t, "FEE", feeCurrency)
	})
}

func Test_feeModeFunctionQuote(t *testing.T) {
	market := getTestMarket()
	t.Run("sellOrder", func(t *testing.T) {
		order := types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:      market.Symbol,
				Side:        types.SideTypeSell,
				Type:        types.OrderTypeLimit,
				Quantity:    fixedpoint.NewFromFloat(0.1),
				Price:       fixedpoint.NewFromFloat(20000.0),
				TimeInForce: types.TimeInForceGTC,
			},
		}
		feeRate := fixedpoint.MustNewFromString("0.075%")
		fee, feeCurrency := feeModeFunctionQuote(&order, &market, feeRate)
		assert.Equal(t, "1.5", fee.String())
		assert.Equal(t, "USDT", feeCurrency)
	})

	t.Run("buyOrder", func(t *testing.T) {
		order := types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:      market.Symbol,
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimit,
				Quantity:    fixedpoint.NewFromFloat(0.1),
				Price:       fixedpoint.NewFromFloat(20000.0),
				TimeInForce: types.TimeInForceGTC,
			},
		}

		feeRate := fixedpoint.MustNewFromString("0.075%")
		fee, feeCurrency := feeModeFunctionQuote(&order, &market, feeRate)
		assert.Equal(t, "1.5", fee.String())
		assert.Equal(t, "USDT", feeCurrency)
	})
}

func Test_feeModeFunctionNative(t *testing.T) {
	market := getTestMarket()
	t.Run("sellOrder", func(t *testing.T) {
		order := types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:      market.Symbol,
				Side:        types.SideTypeSell,
				Type:        types.OrderTypeLimit,
				Quantity:    fixedpoint.NewFromFloat(0.1),
				Price:       fixedpoint.NewFromFloat(20000.0),
				TimeInForce: types.TimeInForceGTC,
			},
		}
		feeRate := fixedpoint.MustNewFromString("0.075%")
		fee, feeCurrency := feeModeFunctionNative(&order, &market, feeRate)
		assert.Equal(t, "1.5", fee.String())
		assert.Equal(t, "USDT", feeCurrency)
	})

	t.Run("buyOrder", func(t *testing.T) {
		order := types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:      market.Symbol,
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimit,
				Quantity:    fixedpoint.NewFromFloat(0.1),
				Price:       fixedpoint.NewFromFloat(20000.0),
				TimeInForce: types.TimeInForceGTC,
			},
		}

		feeRate := fixedpoint.MustNewFromString("0.075%")
		fee, feeCurrency := feeModeFunctionNative(&order, &market, feeRate)
		assert.Equal(t, "0.000075", fee.String())
		assert.Equal(t, "BTC", feeCurrency)
	})
}
