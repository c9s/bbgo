package riskcontrol

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_ModifiedQuantity(t *testing.T) {
	pos := &types.Position{
		Market: types.Market{
			Symbol:          "BTCUSDT",
			PricePrecision:  8,
			VolumePrecision: 8,
			QuoteCurrency:   "USDT",
			BaseCurrency:    "BTC",
		},
	}
	orderExecutor := bbgo.NewGeneralOrderExecutor(nil, "BTCUSDT", "strategy", "strategy-1", pos)
	riskControl := NewPositionRiskControl(orderExecutor, fixedpoint.NewFromInt(10), fixedpoint.NewFromInt(2))

	cases := []struct {
		name         string
		position     fixedpoint.Value
		buyQuantity  fixedpoint.Value
		sellQuantity fixedpoint.Value
	}{
		{
			name:         "BuyOverHardLimit",
			position:     fixedpoint.NewFromInt(9),
			buyQuantity:  fixedpoint.NewFromInt(1),
			sellQuantity: fixedpoint.NewFromInt(2),
		},
		{
			name:         "SellOverHardLimit",
			position:     fixedpoint.NewFromInt(-9),
			buyQuantity:  fixedpoint.NewFromInt(2),
			sellQuantity: fixedpoint.NewFromInt(1),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buyQuantity, sellQuantity := riskControl.ModifiedQuantity(tc.position)
			assert.Equal(t, tc.buyQuantity, buyQuantity)
			assert.Equal(t, tc.sellQuantity, sellQuantity)
		})
	}
}

func TestReleasePositionCallbacks(t *testing.T) {
	cases := []struct {
		name           string
		position       fixedpoint.Value
		resultPosition fixedpoint.Value
	}{
		{
			name:           "PostivePositionWithinLimit",
			position:       fixedpoint.NewFromInt(8),
			resultPosition: fixedpoint.NewFromInt(8),
		},
		{
			name:           "NegativePositionWithinLimit",
			position:       fixedpoint.NewFromInt(-8),
			resultPosition: fixedpoint.NewFromInt(-8),
		},
		{
			name:           "PostivePositionOverLimit",
			position:       fixedpoint.NewFromInt(11),
			resultPosition: fixedpoint.NewFromInt(10),
		},
		{
			name:           "NegativePositionOverLimit",
			position:       fixedpoint.NewFromInt(-11),
			resultPosition: fixedpoint.NewFromInt(-10),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pos := &types.Position{
				Base: tc.position,
				Market: types.Market{
					Symbol:          "BTCUSDT",
					PricePrecision:  8,
					VolumePrecision: 8,
					QuoteCurrency:   "USDT",
					BaseCurrency:    "BTC",
				},
			}

			orderExecutor := bbgo.NewGeneralOrderExecutor(nil, "BTCUSDT", "strategy", "strategy-1", pos)
			riskControl := NewPositionRiskControl(orderExecutor, fixedpoint.NewFromInt(10), fixedpoint.NewFromInt(2))
			riskControl.OnReleasePosition(func(quantity fixedpoint.Value, side types.SideType) {
				if side == types.SideTypeBuy {
					pos.Base = pos.Base.Add(quantity)
				} else {
					pos.Base = pos.Base.Sub(quantity)
				}
			})

			orderExecutor.TradeCollector().EmitPositionUpdate(&types.Position{Base: tc.position})
			assert.Equal(t, tc.resultPosition, pos.Base)
		})
	}
}
