package binance

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestToGlobalPositionRisk(t *testing.T) {
	positions := []binanceapi.FuturesPositionRisk{
		{
			Symbol:                 "BTCUSDT",
			PositionSide:           "LONG",
			MarkPrice:              fixedpoint.MustNewFromString("51000"),
			EntryPrice:             fixedpoint.MustNewFromString("50000"),
			PositionAmount:         fixedpoint.MustNewFromString("1.0"),
			BreakEvenPrice:         fixedpoint.MustNewFromString("49000"),
			UnRealizedProfit:       fixedpoint.MustNewFromString("1000"),
			InitialMargin:          fixedpoint.MustNewFromString("0.1"),
			Notional:               fixedpoint.MustNewFromString("51000"),
			PositionInitialMargin:  fixedpoint.MustNewFromString("0.1"),
			MaintMargin:            fixedpoint.MustNewFromString("0.05"),
			Adl:                    fixedpoint.MustNewFromString("1"),
			OpenOrderInitialMargin: fixedpoint.MustNewFromString("0.1"),
			UpdateTime:             types.MillisecondTimestamp(time.Unix(1234567890/1000, 0)),
			MarginAsset:            "USDT",
		},
	}

	risks := toGlobalPositionRisk(positions)
	assert.NotNil(t, risks)
	assert.Len(t, risks, 1)

	risk := risks[0]
	assert.Equal(t, "BTCUSDT", risk.Symbol)
	assert.Equal(t, types.PositionType("LONG"), risk.PositionSide)
	assert.Equal(t, fixedpoint.MustNewFromString("51000"), risk.MarkPrice)
	assert.Equal(t, fixedpoint.MustNewFromString("50000"), risk.EntryPrice)
	assert.Equal(t, fixedpoint.MustNewFromString("1.0"), risk.PositionAmount)
	assert.Equal(t, fixedpoint.MustNewFromString("49000"), risk.BreakEvenPrice)
	assert.Equal(t, fixedpoint.MustNewFromString("1000"), risk.UnrealizedPnL)
	assert.Equal(t, fixedpoint.MustNewFromString("0.1"), risk.InitialMargin)
	assert.Equal(t, fixedpoint.MustNewFromString("51000"), risk.Notional)
	assert.Equal(t, fixedpoint.MustNewFromString("0.1"), risk.PositionInitialMargin)
	assert.Equal(t, fixedpoint.MustNewFromString("0.05"), risk.MaintMargin)
	assert.Equal(t, fixedpoint.MustNewFromString("1"), risk.Adl)
	assert.Equal(t, fixedpoint.MustNewFromString("0.1"), risk.OpenOrderInitialMargin)
	assert.Equal(t, types.MillisecondTimestamp(time.Unix(1234567890/1000, 0)), risk.UpdateTime)
	assert.Equal(t, "USDT", risk.MarginAsset)
}
