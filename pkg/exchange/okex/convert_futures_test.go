package okex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_toGlobalFuturesAccountInfo(t *testing.T) {
	testTime := time.Now()
	account := &okexapi.Account{
		TotalEquityInUSD:            fixedpoint.NewFromFloat(1000.0),
		TotalInitialMargin:          fixedpoint.NewFromFloat(100.0),
		TotalMaintMargin:            fixedpoint.NewFromFloat(50.0),
		TotalOpenOrderInitialMargin: fixedpoint.NewFromFloat(20.0),
		UnrealizedPnl:               fixedpoint.NewFromFloat(30.0),
		Details: []okexapi.BalanceDetail{
			{
				Currency:                "BTC",
				Equity:                  fixedpoint.NewFromFloat(1.0),
				EquityInUSD:             fixedpoint.NewFromFloat(500.0),
				Available:               fixedpoint.NewFromFloat(0.8),
				Imr:                     "50.0",
				Mmr:                     "25.0",
				UnrealizedProfitAndLoss: fixedpoint.NewFromFloat(15.0),
			},
		},
	}

	positions := []okexapi.Position{
		{
			InstId:      "BTC-USDT-SWAP",
			MgnMode:     okexapi.MarginModeIsolated,
			Pos:         fixedpoint.NewFromFloat(0.1),
			AvgPx:       fixedpoint.NewFromFloat(50000.0),
			MarkPx:      fixedpoint.NewFromFloat(51000.0),
			Lever:       fixedpoint.NewFromFloat(10.0),
			UpdatedTime: types.MillisecondTimestamp(testTime),
		},
	}

	info := toGlobalFuturesAccountInfo(account, positions)

	assert.Equal(t, account.TotalEquityInUSD, info.TotalMarginBalance)
	assert.Equal(t, account.TotalInitialMargin, info.TotalInitialMargin)
	assert.Equal(t, account.TotalMaintMargin, info.TotalMaintMargin)
	assert.Equal(t, account.UnrealizedPnl, info.TotalUnrealizedProfit)

	// Verify asset conversion
	btcAsset, exists := info.Assets["BTC"]
	assert.True(t, exists)
	assert.Equal(t, "BTC", btcAsset.Asset)
	assert.Equal(t, fixedpoint.MustNewFromString("50.0"), btcAsset.InitialMargin)
	assert.Equal(t, fixedpoint.MustNewFromString("25.0"), btcAsset.MaintMargin)
	assert.Equal(t, fixedpoint.NewFromFloat(500.0), btcAsset.MarginBalance)

	// Verify position conversion
	position, exists := info.Positions["BTCUSDT"]
	assert.True(t, exists)
	assert.True(t, position.Isolated)
	assert.Equal(t, fixedpoint.NewFromFloat(0.1), position.Base)
	assert.Equal(t, fixedpoint.NewFromFloat(50000.0), position.AverageCost)
	assert.Equal(t, fixedpoint.NewFromFloat(10.0), position.PositionRisk.Leverage)
}

func Test_toGlobalFuturesUserAssets(t *testing.T) {
	assets := []okexapi.BalanceDetail{
		{
			Currency:                "ETH",
			Equity:                  fixedpoint.NewFromFloat(2.0),
			EquityInUSD:             fixedpoint.NewFromFloat(4000.0),
			Available:               fixedpoint.NewFromFloat(1.5),
			Imr:                     "400.0",
			Mmr:                     "200.0",
			UnrealizedProfitAndLoss: fixedpoint.NewFromFloat(100.0),
		},
	}

	globalAssets := toGlobalFuturesUserAssets(assets)

	ethAsset, exists := globalAssets["ETH"]
	assert.True(t, exists)
	assert.Equal(t, "ETH", ethAsset.Asset)
	assert.Equal(t, fixedpoint.MustNewFromString("400.0"), ethAsset.InitialMargin)
	assert.Equal(t, fixedpoint.MustNewFromString("200.0"), ethAsset.MaintMargin)
	assert.Equal(t, fixedpoint.NewFromFloat(4000.0), ethAsset.MarginBalance)
	assert.Equal(t, fixedpoint.NewFromFloat(1.5), ethAsset.MaxWithdrawAmount)
	assert.Equal(t, fixedpoint.NewFromFloat(100.0), ethAsset.UnrealizedProfit)
}

func Test_toGlobalFuturesPositions(t *testing.T) {

	testTime := time.Now()
	positions := []okexapi.Position{
		{
			InstId:      "ETH-USDT-SWAP",
			MgnMode:     okexapi.MarginModeCross,
			Pos:         fixedpoint.NewFromFloat(1.0),
			AvgPx:       fixedpoint.NewFromFloat(2000.0),
			MarkPx:      fixedpoint.NewFromFloat(2100.0),
			Lever:       fixedpoint.NewFromFloat(5.0),
			UpdatedTime: types.MillisecondTimestamp(testTime),
		},
	}

	globalPositions := toGlobalFuturesPositions(positions)

	ethPosition, exists := globalPositions["ETHUSDT"]
	assert.True(t, exists)
	assert.False(t, ethPosition.Isolated) // Cross margin
	assert.Equal(t, fixedpoint.NewFromFloat(1.0), ethPosition.Base)
	assert.Equal(t, fixedpoint.NewFromFloat(2000.0), ethPosition.AverageCost)
	assert.Equal(t, fixedpoint.NewFromFloat(2100.0), ethPosition.Quote.Div(ethPosition.Base))
	assert.Equal(t, fixedpoint.NewFromFloat(5.0), ethPosition.PositionRisk.Leverage)
}

func TestToGlobalPositionSide(t *testing.T) {
	assert.Equal(t, types.PositionLong, toGlobalPositionSide(okexapi.PosSideLong))
	assert.Equal(t, types.PositionShort, toGlobalPositionSide(okexapi.PosSideShort))
	assert.Equal(t, types.PositionType("BOTH"), toGlobalPositionSide(okexapi.PosSide("BOTH")))
}

func TestToGlobalPositionRisk(t *testing.T) {
	positions := []okexapi.Position{
		{
			InstId:      "BTC-USDT-SWAP",
			PosSide:     "long",
			MarkPx:      fixedpoint.MustNewFromString("51000"),
			AvgPx:       fixedpoint.MustNewFromString("50000"),
			BePx:        fixedpoint.MustNewFromString("49000"),
			Upl:         "1000",
			Imr:         "0.1",
			Mmr:         "0.05",
			NotionalUsd: "51000",
			Lever:       fixedpoint.MustNewFromString("10"),
			Ccy:         "USDT",
			Adl:         fixedpoint.MustNewFromString("1"),
			UpdatedTime: types.MillisecondTimestamp(time.Unix(1234567890/1000, 0)),
		},
	}

	risks := toGlobalPositionRisk(positions)
	assert.NotNil(t, risks)
	assert.Len(t, risks, 1)

	risk := risks[0]
	assert.Equal(t, "BTCUSDT", risk.Symbol)
	assert.Equal(t, types.PositionLong, risk.PositionSide)
	assert.Equal(t, fixedpoint.MustNewFromString("51000"), risk.MarkPrice)
	assert.Equal(t, fixedpoint.MustNewFromString("50000"), risk.EntryPrice)
	assert.Equal(t, fixedpoint.MustNewFromString("49000"), risk.BreakEvenPrice)
	assert.Equal(t, fixedpoint.MustNewFromString("1000"), risk.UnrealizedPnL)
	assert.Equal(t, fixedpoint.MustNewFromString("0.1"), risk.InitialMargin)
	assert.Equal(t, fixedpoint.MustNewFromString("0.05"), risk.MaintMargin)
	assert.Equal(t, fixedpoint.MustNewFromString("51000"), risk.Notional)
	assert.Equal(t, fixedpoint.MustNewFromString("10"), risk.Leverage)
	assert.Equal(t, "USDT", risk.MarginAsset)
	assert.Equal(t, fixedpoint.MustNewFromString("1"), risk.Adl)
}
