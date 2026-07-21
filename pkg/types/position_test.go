package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

const Delta = 1e-9

func TestPosition_ROI(t *testing.T) {
	t.Run("short position", func(t *testing.T) {
		// Long position
		pos := &Position{
			Symbol:             "BTCUSDT",
			BaseCurrency:       "BTC",
			QuoteCurrency:      "USDT",
			Base:               fixedpoint.NewFromFloat(-10.0),
			AverageCost:        fixedpoint.NewFromFloat(8000.0),
			Quote:              fixedpoint.NewFromFloat(8000.0 * 10.0),
			StrategyInstanceID: "test-position:BTCUSDT",
			Strategy:           "test-position",
		}

		assert.True(t, pos.IsShort(), "should be a short position")

		currentPrice := fixedpoint.NewFromFloat(5000.0)
		roi := pos.ROI(currentPrice)
		assert.Equal(t, "0.375", roi.String())
		assert.Equal(t, "37.5%", roi.Percentage())
	})

	t.Run("long position", func(t *testing.T) {
		// Long position
		pos := &Position{
			Symbol:             "BTCUSDT",
			BaseCurrency:       "BTC",
			QuoteCurrency:      "USDT",
			Base:               fixedpoint.NewFromFloat(10.0),
			AverageCost:        fixedpoint.NewFromFloat(8000.0),
			Quote:              fixedpoint.NewFromFloat(-8000.0 * 10.0),
			StrategyInstanceID: "test-position:BTCUSDT",
			Strategy:           "test-position",
		}

		assert.True(t, pos.IsLong(), "should be a long position")

		currentPrice := fixedpoint.NewFromFloat(10000.0)
		roi := pos.ROI(currentPrice)
		assert.Equal(t, "0.25", roi.String())
		assert.Equal(t, "25%", roi.Percentage())
	})
}

func TestPosition_ExchangeFeeRate_Short(t *testing.T) {
	pos := &Position{
		Symbol:             "BTCUSDT",
		BaseCurrency:       "BTC",
		QuoteCurrency:      "USDT",
		StrategyInstanceID: "test-position:BTCUSDT",
		Strategy:           "test-position",
	}

	feeRate := fixedpoint.NewFromFloat(0.075 * 0.01)
	pos.SetExchangeFeeRate(ExchangeBinance, ExchangeFee{
		MakerFeeRate: feeRate,
		TakerFeeRate: feeRate,
	})

	quantity := fixedpoint.NewFromInt(10)
	quoteQuantity := fixedpoint.NewFromInt(3000).Mul(quantity)
	fee := quoteQuantity.Mul(feeRate)
	averageCost := quoteQuantity.Sub(fee).Div(quantity)
	bnbPrice := fixedpoint.NewFromInt(570)
	pos.AddTrade(Trade{
		Exchange:      ExchangeBinance,
		Price:         fixedpoint.NewFromInt(3000),
		Quantity:      quantity,
		QuoteQuantity: quoteQuantity,
		Symbol:        "BTCUSDT",
		Side:          SideTypeSell,
		Fee:           fee.Div(bnbPrice),
		FeeCurrency:   "BNB",
	})

	_, netProfit, madeProfit := pos.AddTrade(Trade{
		Exchange:      ExchangeBinance,
		Price:         fixedpoint.NewFromInt(2000),
		Quantity:      fixedpoint.NewFromInt(10),
		QuoteQuantity: fixedpoint.NewFromInt(2000 * 10),
		Symbol:        "BTCUSDT",
		Side:          SideTypeBuy,
		Fee:           fixedpoint.NewFromInt(2000 * 10.0).Mul(feeRate).Div(bnbPrice),
		FeeCurrency:   "BNB",
	})

	expectedProfit := averageCost.Sub(fixedpoint.NewFromInt(2000)).
		Mul(fixedpoint.NewFromInt(10)).
		Sub(fixedpoint.NewFromInt(2000).Mul(fixedpoint.NewFromInt(10)).Mul(feeRate))
	assert.True(t, madeProfit)
	assert.Equal(t, expectedProfit, netProfit)
}

func TestPosition_ExchangeFeeRate_Long(t *testing.T) {
	pos := &Position{
		Symbol:             "BTCUSDT",
		BaseCurrency:       "BTC",
		QuoteCurrency:      "USDT",
		StrategyInstanceID: "test-position:BTCUSDT",
		Strategy:           "test-position",
	}

	feeRate := fixedpoint.NewFromFloat(0.075 * 0.01)
	pos.SetExchangeFeeRate(ExchangeBinance, ExchangeFee{
		MakerFeeRate: feeRate,
		TakerFeeRate: feeRate,
	})

	quantity := fixedpoint.NewFromInt(10)
	quoteQuantity := fixedpoint.NewFromInt(3000).Mul(quantity)
	fee := quoteQuantity.Mul(feeRate)
	averageCost := quoteQuantity.Add(fee).Div(quantity)
	bnbPrice := fixedpoint.NewFromInt(570)
	pos.AddTrade(Trade{
		Exchange:      ExchangeBinance,
		Price:         fixedpoint.NewFromInt(3000),
		Quantity:      quantity,
		QuoteQuantity: quoteQuantity,
		Symbol:        "BTCUSDT",
		Side:          SideTypeBuy,
		Fee:           fee.Div(bnbPrice),
		FeeCurrency:   "BNB",
	})

	_, netProfit, madeProfit := pos.AddTrade(Trade{
		Exchange:      ExchangeBinance,
		Price:         fixedpoint.NewFromInt(4000),
		Quantity:      fixedpoint.NewFromInt(10),
		QuoteQuantity: fixedpoint.NewFromInt(4000).Mul(fixedpoint.NewFromInt(10)),
		Symbol:        "BTCUSDT",
		Side:          SideTypeSell,
		Fee:           fixedpoint.NewFromInt(40000).Mul(feeRate).Div(bnbPrice),
		FeeCurrency:   "BNB",
	})

	expectedProfit := fixedpoint.NewFromInt(4000).
		Sub(averageCost).Mul(fixedpoint.NewFromInt(10)).
		Sub(fixedpoint.NewFromInt(40000).Mul(feeRate))
	assert.True(t, madeProfit)
	assert.Equal(t, expectedProfit, netProfit)
}

func TestPosition(t *testing.T) {
	var feeRate float64 = 0.05 * 0.01
	feeRateValue := fixedpoint.NewFromFloat(feeRate)
	var testcases = []struct {
		name                string
		trades              []Trade
		expectedAverageCost fixedpoint.Value
		expectedBase        fixedpoint.Value
		expectedQuote       fixedpoint.Value
		expectedProfit      fixedpoint.Value
	}{
		{
			name: "base fee",
			trades: []Trade{
				{
					Side:          SideTypeBuy,
					Price:         fixedpoint.NewFromInt(1000),
					Quantity:      fixedpoint.NewFromFloat(0.01),
					QuoteQuantity: fixedpoint.NewFromFloat(1000.0 * 0.01),
					Fee:           fixedpoint.MustNewFromString("0.000005"), // 0.01 * 0.05 * 0.01
					FeeCurrency:   "BTC",
				},
			},
			expectedAverageCost: fixedpoint.NewFromFloat(1000.0 * 0.01).
				Div(fixedpoint.NewFromFloat(0.01).Mul(fixedpoint.One.Sub(feeRateValue))),
			expectedBase: fixedpoint.NewFromFloat(0.01).
				Sub(fixedpoint.NewFromFloat(0.01).Mul(feeRateValue)),
			expectedQuote:  fixedpoint.NewFromFloat(0 - 1000.0*0.01),
			expectedProfit: fixedpoint.Zero,
		},
		{
			name: "quote fee",
			trades: []Trade{
				{
					Side:          SideTypeSell,
					Price:         fixedpoint.NewFromInt(1000),
					Quantity:      fixedpoint.NewFromFloat(0.01),
					QuoteQuantity: fixedpoint.NewFromFloat(1000.0 * 0.01),
					Fee:           fixedpoint.NewFromFloat((1000.0 * 0.01) * feeRate), // 0.05%
					FeeCurrency:   "USDT",
				},
			},
			expectedAverageCost: fixedpoint.NewFromFloat(1000.0 * 0.01).
				Mul(fixedpoint.One.Sub(feeRateValue)).
				Div(fixedpoint.NewFromFloat(0.01)),
			expectedBase:   fixedpoint.NewFromFloat(-0.01),
			expectedQuote:  fixedpoint.NewFromFloat(0.0 + 1000.0*0.01*(1.0-feeRate)),
			expectedProfit: fixedpoint.Zero,
		},
		{
			name: "long",
			trades: []Trade{
				{
					Side:          SideTypeBuy,
					Price:         fixedpoint.NewFromInt(1000),
					Quantity:      fixedpoint.NewFromFloat(0.01),
					QuoteQuantity: fixedpoint.NewFromFloat(1000.0 * 0.01),
				},
				{
					Side:          SideTypeBuy,
					Price:         fixedpoint.NewFromInt(2000),
					Quantity:      fixedpoint.MustNewFromString("0.03"),
					QuoteQuantity: fixedpoint.NewFromFloat(2000.0 * 0.03),
				},
			},
			expectedAverageCost: fixedpoint.NewFromFloat((1000.0*0.01 + 2000.0*0.03) / 0.04),
			expectedBase:        fixedpoint.NewFromFloat(0.01 + 0.03),
			expectedQuote:       fixedpoint.NewFromFloat(0 - 1000.0*0.01 - 2000.0*0.03),
			expectedProfit:      fixedpoint.Zero,
		},

		{
			name: "long and sell",
			trades: []Trade{
				{
					Side:          SideTypeBuy,
					Price:         fixedpoint.NewFromInt(1000),
					Quantity:      fixedpoint.NewFromFloat(0.01),
					QuoteQuantity: fixedpoint.NewFromFloat(1000.0 * 0.01),
				},
				{
					Side:          SideTypeBuy,
					Price:         fixedpoint.NewFromInt(2000),
					Quantity:      fixedpoint.MustNewFromString("0.03"),
					QuoteQuantity: fixedpoint.NewFromFloat(2000.0 * 0.03),
				},
				{
					Side:          SideTypeSell,
					Price:         fixedpoint.NewFromInt(3000),
					Quantity:      fixedpoint.NewFromFloat(0.01),
					QuoteQuantity: fixedpoint.NewFromFloat(3000.0 * 0.01),
				},
			},
			expectedAverageCost: fixedpoint.NewFromFloat((1000.0*0.01 + 2000.0*0.03) / 0.04),
			expectedBase:        fixedpoint.MustNewFromString("0.03"),
			expectedQuote:       fixedpoint.NewFromFloat(0 - 1000.0*0.01 - 2000.0*0.03 + 3000.0*0.01),
			expectedProfit:      fixedpoint.NewFromFloat((3000.0 - (1000.0*0.01+2000.0*0.03)/0.04) * 0.01),
		},

		{
			name: "long and sell to short",
			trades: []Trade{
				{
					Side:          SideTypeBuy,
					Price:         fixedpoint.NewFromInt(1000),
					Quantity:      fixedpoint.NewFromFloat(0.01),
					QuoteQuantity: fixedpoint.NewFromFloat(1000.0 * 0.01),
				},
				{
					Side:          SideTypeBuy,
					Price:         fixedpoint.NewFromInt(2000),
					Quantity:      fixedpoint.MustNewFromString("0.03"),
					QuoteQuantity: fixedpoint.NewFromFloat(2000.0 * 0.03),
				},
				{
					Side:          SideTypeSell,
					Price:         fixedpoint.NewFromInt(3000),
					Quantity:      fixedpoint.NewFromFloat(0.10),
					QuoteQuantity: fixedpoint.NewFromFloat(3000.0 * 0.10),
				},
			},

			expectedAverageCost: fixedpoint.NewFromInt(3000),
			expectedBase:        fixedpoint.MustNewFromString("-0.06"),
			expectedQuote:       fixedpoint.NewFromFloat(-1000.0*0.01 - 2000.0*0.03 + 3000.0*0.1),
			expectedProfit:      fixedpoint.NewFromFloat((3000.0 - (1000.0*0.01+2000.0*0.03)/0.04) * 0.04),
		},

		{
			name: "short",
			trades: []Trade{
				{
					Side:          SideTypeSell,
					Price:         fixedpoint.NewFromInt(2000),
					Quantity:      fixedpoint.NewFromFloat(0.01),
					QuoteQuantity: fixedpoint.NewFromFloat(2000.0 * 0.01),
				},
				{
					Side:          SideTypeSell,
					Price:         fixedpoint.NewFromInt(3000),
					Quantity:      fixedpoint.MustNewFromString("0.03"),
					QuoteQuantity: fixedpoint.NewFromFloat(3000.0 * 0.03),
				},
			},

			expectedAverageCost: fixedpoint.NewFromFloat((2000.0*0.01 + 3000.0*0.03) / (0.01 + 0.03)),
			expectedBase:        fixedpoint.NewFromFloat(0 - 0.01 - 0.03),
			expectedQuote:       fixedpoint.NewFromFloat(2000.0*0.01 + 3000.0*0.03),
			expectedProfit:      fixedpoint.Zero,
		},

		{
			name: "different symbols",
			trades: []Trade{
				{
					Symbol:        "BTCUSDT",
					Side:          SideTypeBuy,
					Price:         fixedpoint.NewFromInt(1000),
					Quantity:      fixedpoint.NewFromFloat(0.01),
					QuoteQuantity: fixedpoint.NewFromFloat(1000.0 * 0.01),
					Fee:           fixedpoint.Zero,
					FeeCurrency:   "BTC",
				},
				{
					Symbol:        "BTCUSD",
					Side:          SideTypeSell,
					Price:         fixedpoint.NewFromInt(1020),
					Quantity:      fixedpoint.NewFromFloat(0.01),
					QuoteQuantity: fixedpoint.NewFromFloat(1020.0 * 0.01),
					Fee:           fixedpoint.Zero,
					FeeCurrency:   "BTC",
				},
			},
			expectedAverageCost: fixedpoint.NewFromFloat(1000.0 * 0.01).
				Div(fixedpoint.NewFromFloat(0.01)),
			expectedBase:   fixedpoint.NewFromFloat(0.0),
			expectedQuote:  fixedpoint.NewFromFloat(20.0 * 0.01),
			expectedProfit: fixedpoint.NewFromFloat(0.2),
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			pos := Position{
				Symbol:             "BTCUSDT",
				BaseCurrency:       "BTC",
				QuoteCurrency:      "USDT",
				StrategyInstanceID: "test-position:BTCUSDT",
				Strategy:           "test-position",
			}
			profitAmount, _, profit := pos.AddTrades(testcase.trades)
			assert.InDelta(t, testcase.expectedQuote.Float64(), pos.Quote.Float64(), 1e-3, "expectedQuote")
			assert.InDelta(t, testcase.expectedBase.Float64(), pos.Base.Float64(), 1e-8, "expectedBase")
			assert.InDelta(t, testcase.expectedAverageCost.Float64(), pos.AverageCost.Float64(), 1e-8, "expectedAverageCost")
			if profit {
				assert.InDelta(t, testcase.expectedProfit.Float64(), profitAmount.Float64(), 1e-8, "expectedProfit")
			}
		})
	}
}

func TestPosition_SetClosing(t *testing.T) {
	p := NewPosition("BTCUSDT", "BTC", "USDT")
	ret := p.SetClosing(true)
	assert.True(t, ret)

	ret = p.SetClosing(true)
	assert.False(t, ret)

	ret = p.SetClosing(false)
	assert.True(t, ret)
}

func TestPosition_GetBaseAndAverageCost(t *testing.T) {
	pos := Position{
		Symbol:             "BTCUSDT",
		BaseCurrency:       "BTC",
		QuoteCurrency:      "USDT",
		Base:               fixedpoint.NewFromFloat(0.01),
		AverageCost:        fixedpoint.NewFromFloat(1000),
		StrategyInstanceID: "test-position:BTCUSDT",
		Strategy:           "test-position",
	}
	base, avgCost := pos.GetBaseAndAverageCost()
	assert.Equal(t, pos.Base, base)
	assert.Equal(t, pos.AverageCost, avgCost)
}

func TestPosition_ExcludeFeeFromCostMode(t *testing.T) {
	// unit tests for the PnL calculation when the PnLMode is set to ExcludeFeeFromCost
	newPosition := func() *Position {
		pos := NewPosition("BTCUSDT", "BTC", "USDT")
		pos.Strategy = "test-position"
		pos.StrategyInstanceID = "test-position:BTCUSDT"
		pos.UseExcludeFeeFromCostMode()
		return pos
	}

	t.Run("open long, quote fee excluded from average cost", func(t *testing.T) {
		pos := newPosition()

		// buy 0.01 BTC @ 1000 with a 0.01 USDT (quote) fee
		profit, netProfit, madeProfit := pos.AddTrade(Trade{
			Side:          SideTypeBuy,
			Price:         fixedpoint.NewFromInt(1000),
			Quantity:      fixedpoint.NewFromFloat(0.01),
			QuoteQuantity: fixedpoint.NewFromFloat(10.0),
			Fee:           fixedpoint.NewFromFloat(0.01),
			FeeCurrency:   "USDT",
		})

		// opening a position realizes no gross PnL, but the fee is booked as a net loss
		assert.True(t, madeProfit)
		assert.InDelta(t, 0.0, profit.Float64(), Delta, "profit")
		assert.InDelta(t, -0.01, netProfit.Float64(), Delta, "netProfit")

		// average cost must NOT include the fee (1000, not 1001)
		assert.InDelta(t, 1000.0, pos.AverageCost.Float64(), Delta, "averageCost")
		assert.InDelta(t, 0.01, pos.Base.Float64(), Delta, "base")
		assert.InDelta(t, -10.0, pos.Quote.Float64(), Delta, "quote")
	})

	t.Run("open long, base fee does not reduce base quantity", func(t *testing.T) {
		pos := newPosition()

		// buy 0.01 BTC @ 1000 with a 0.00001 BTC (base) fee => 0.01 USDT
		profit, netProfit, madeProfit := pos.AddTrade(Trade{
			Side:          SideTypeBuy,
			Price:         fixedpoint.NewFromInt(1000),
			Quantity:      fixedpoint.NewFromFloat(0.01),
			QuoteQuantity: fixedpoint.NewFromFloat(10.0),
			Fee:           fixedpoint.NewFromFloat(0.00001),
			FeeCurrency:   "BTC",
		})

		assert.True(t, madeProfit)
		assert.InDelta(t, 0.0, profit.Float64(), Delta, "profit")
		assert.InDelta(t, -0.01, netProfit.Float64(), Delta, "netProfit")

		// unlike the classic mode, the base fee is not subtracted from the acquired base
		assert.InDelta(t, 0.01, pos.Base.Float64(), Delta, "base")
		// and the average cost stays exactly the trade price
		assert.InDelta(t, 1000.0, pos.AverageCost.Float64(), Delta, "averageCost")
	})

	t.Run("long then close with profit", func(t *testing.T) {
		pos := newPosition()

		pos.AddTrade(Trade{
			Side:          SideTypeBuy,
			Price:         fixedpoint.NewFromInt(1000),
			Quantity:      fixedpoint.NewFromFloat(0.01),
			QuoteQuantity: fixedpoint.NewFromFloat(10.0),
			Fee:           fixedpoint.NewFromFloat(0.01),
			FeeCurrency:   "USDT",
		})

		// sell 0.01 BTC @ 1100 with 0.011 USDT fee => close the long
		profit, netProfit, madeProfit := pos.AddTrade(Trade{
			Side:          SideTypeSell,
			Price:         fixedpoint.NewFromInt(1100),
			Quantity:      fixedpoint.NewFromFloat(0.01),
			QuoteQuantity: fixedpoint.NewFromFloat(11.0),
			Fee:           fixedpoint.NewFromFloat(0.011),
			FeeCurrency:   "USDT",
		})

		assert.True(t, madeProfit)
		// gross realized PnL = (1100 - 1000) * 0.01 = 1.0
		assert.InDelta(t, 1.0, profit.Float64(), Delta, "profit")
		// net realized PnL = 1.0 - 0.011 (sell fee only, cost carries no fee) = 0.989
		assert.InDelta(t, 0.989, netProfit.Float64(), Delta, "netProfit")

		assert.InDelta(t, 0.0, pos.Base.Float64(), Delta, "base")
		assert.InDelta(t, 1.0, pos.AccumulatedProfit.Float64(), Delta, "accumulatedProfit")
	})

	t.Run("short then cover with profit", func(t *testing.T) {
		pos := newPosition()

		// open short: sell 0.01 BTC @ 2000
		pos.AddTrade(Trade{
			Side:          SideTypeSell,
			Price:         fixedpoint.NewFromInt(2000),
			Quantity:      fixedpoint.NewFromFloat(0.01),
			QuoteQuantity: fixedpoint.NewFromFloat(20.0),
			Fee:           fixedpoint.NewFromFloat(0.02),
			FeeCurrency:   "USDT",
		})
		assert.InDelta(t, 2000.0, pos.AverageCost.Float64(), Delta, "averageCost after open short")
		assert.InDelta(t, -0.01, pos.Base.Float64(), Delta, "base after open short")

		// cover: buy 0.01 BTC @ 1800
		profit, netProfit, madeProfit := pos.AddTrade(Trade{
			Side:          SideTypeBuy,
			Price:         fixedpoint.NewFromInt(1800),
			Quantity:      fixedpoint.NewFromFloat(0.01),
			QuoteQuantity: fixedpoint.NewFromFloat(18.0),
			Fee:           fixedpoint.NewFromFloat(0.018),
			FeeCurrency:   "USDT",
		})

		assert.True(t, madeProfit)
		// gross realized PnL = (2000 - 1800) * 0.01 = 2.0
		assert.InDelta(t, 2.0, profit.Float64(), Delta, "profit")
		// net realized PnL = 2.0 - 0.018 = 1.982
		assert.InDelta(t, 1.982, netProfit.Float64(), 1e-8, "netProfit")

		assert.InDelta(t, 0.0, pos.Base.Float64(), Delta, "base")
		assert.InDelta(t, 2.0, pos.AccumulatedProfit.Float64(), Delta, "accumulatedProfit")
	})

	t.Run("long flips to short and realizes profit on the closed portion", func(t *testing.T) {
		pos := newPosition()

		// open long: buy 0.02 BTC @ 1000
		pos.AddTrade(Trade{
			Side:          SideTypeBuy,
			Price:         fixedpoint.NewFromInt(1000),
			Quantity:      fixedpoint.NewFromFloat(0.02),
			QuoteQuantity: fixedpoint.NewFromFloat(20.0),
			Fee:           fixedpoint.NewFromFloat(0.02),
			FeeCurrency:   "USDT",
		})

		// sell 0.03 BTC @ 1200 => close 0.02 long and open 0.01 short
		profit, netProfit, madeProfit := pos.AddTrade(Trade{
			Side:          SideTypeSell,
			Price:         fixedpoint.NewFromInt(1200),
			Quantity:      fixedpoint.NewFromFloat(0.03),
			QuoteQuantity: fixedpoint.NewFromFloat(36.0),
			Fee:           fixedpoint.NewFromFloat(0.036),
			FeeCurrency:   "USDT",
		})

		assert.True(t, madeProfit)
		// only the closed long portion realizes PnL: (1200 - 1000) * 0.02 = 4.0
		assert.InDelta(t, 4.0, profit.Float64(), Delta, "profit")
		// net realized PnL = 4.0 - 0.036 = 3.964
		assert.InDelta(t, 3.964, netProfit.Float64(), 1e-8, "netProfit")

		// the remaining short is re-based at the flip price
		assert.InDelta(t, 1200.0, pos.AverageCost.Float64(), Delta, "averageCost")
		assert.InDelta(t, -0.01, pos.Base.Float64(), Delta, "base")
		assert.InDelta(t, 4.0, pos.AccumulatedProfit.Float64(), Delta, "accumulatedProfit")
	})

	t.Run("scaling into a long averages cost without folding in fees", func(t *testing.T) {
		pos := newPosition()

		pos.AddTrade(Trade{
			Side:          SideTypeBuy,
			Price:         fixedpoint.NewFromInt(1000),
			Quantity:      fixedpoint.NewFromFloat(0.01),
			QuoteQuantity: fixedpoint.NewFromFloat(10.0),
			Fee:           fixedpoint.NewFromFloat(0.01),
			FeeCurrency:   "USDT",
		})
		pos.AddTrade(Trade{
			Side:          SideTypeBuy,
			Price:         fixedpoint.NewFromInt(2000),
			Quantity:      fixedpoint.NewFromFloat(0.03),
			QuoteQuantity: fixedpoint.NewFromFloat(60.0),
			Fee:           fixedpoint.NewFromFloat(0.06),
			FeeCurrency:   "USDT",
		})

		// average cost = (10 + 60) / 0.04 = 1750, fees excluded entirely
		assert.InDelta(t, 1750.0, pos.AverageCost.Float64(), Delta, "averageCost")
		assert.InDelta(t, 0.04, pos.Base.Float64(), Delta, "base")
		assert.InDelta(t, -70.0, pos.Quote.Float64(), Delta, "quote")
	})
}
