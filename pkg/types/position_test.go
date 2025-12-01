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
