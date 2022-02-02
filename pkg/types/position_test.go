package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

const Delta = 1e-9

func TestPosition_ExchangeFeeRate_Short(t *testing.T) {
	pos := &Position{
		Symbol:        "BTCUSDT",
		BaseCurrency:  "BTC",
		QuoteCurrency: "USDT",
	}

	feeRate := 0.075 * 0.01
	pos.SetExchangeFeeRate(ExchangeBinance, ExchangeFee{
		MakerFeeRate: fixedpoint.NewFromFloat(feeRate),
		TakerFeeRate: fixedpoint.NewFromFloat(feeRate),
	})

	quantity := 10.0
	quoteQuantity := 3000.0 * quantity
	fee := quoteQuantity * feeRate
	averageCost := (quoteQuantity - fee) / quantity
	bnbPrice := 570.0
	pos.AddTrade(Trade{
		Exchange:      ExchangeBinance,
		Price:         3000.0,
		Quantity:      quantity,
		QuoteQuantity: quoteQuantity,
		Symbol:        "BTCUSDT",
		Side:          SideTypeSell,
		Fee:           fee / bnbPrice,
		FeeCurrency:   "BNB",
	})

	_, netProfit, madeProfit := pos.AddTrade(Trade{
		Exchange:      ExchangeBinance,
		Price:         2000.0,
		Quantity:      10.0,
		QuoteQuantity: 2000.0 * 10.0,
		Symbol:        "BTCUSDT",
		Side:          SideTypeBuy,
		Fee:           2000.0 * 10.0 * feeRate / bnbPrice,
		FeeCurrency:   "BNB",
	})

	expectedProfit := (averageCost-2000.0)*10.0 - (2000.0 * 10.0 * feeRate)
	assert.True(t, madeProfit)
	assert.InDelta(t, expectedProfit, netProfit.Float64(), Delta)
}

func TestPosition_ExchangeFeeRate_Long(t *testing.T) {
	pos := &Position{
		Symbol:        "BTCUSDT",
		BaseCurrency:  "BTC",
		QuoteCurrency: "USDT",
	}

	feeRate := 0.075 * 0.01
	pos.SetExchangeFeeRate(ExchangeBinance, ExchangeFee{
		MakerFeeRate: fixedpoint.NewFromFloat(feeRate),
		TakerFeeRate: fixedpoint.NewFromFloat(feeRate),
	})

	quantity := 10.0
	quoteQuantity := 3000.0 * quantity
	fee := quoteQuantity * feeRate
	averageCost := (quoteQuantity + fee) / quantity
	bnbPrice := 570.0
	pos.AddTrade(Trade{
		Exchange:      ExchangeBinance,
		Price:         3000.0,
		Quantity:      quantity,
		QuoteQuantity: quoteQuantity,
		Symbol:        "BTCUSDT",
		Side:          SideTypeBuy,
		Fee:           fee / bnbPrice,
		FeeCurrency:   "BNB",
	})

	_, netProfit, madeProfit := pos.AddTrade(Trade{
		Exchange:      ExchangeBinance,
		Price:         4000.0,
		Quantity:      10.0,
		QuoteQuantity: 4000.0 * 10.0,
		Symbol:        "BTCUSDT",
		Side:          SideTypeSell,
		Fee:           4000.0 * 10.0 * feeRate / bnbPrice,
		FeeCurrency:   "BNB",
	})

	expectedProfit := (4000.0-averageCost)*10.0 - (4000.0 * 10.0 * feeRate)
	assert.True(t, madeProfit)
	assert.InDelta(t, expectedProfit, netProfit.Float64(), Delta)
}

func TestPosition(t *testing.T) {
	var feeRate float64 = 0.05 * 0.01
	var testcases = []struct {
		name                string
		trades              []Trade
		expectedAverageCost float64
		expectedBase        float64
		expectedQuote       float64
		expectedProfit      float64
	}{
		{
			name: "base fee",
			trades: []Trade{
				{
					Side:          SideTypeBuy,
					Price:         1000.0,
					Quantity:      0.01,
					QuoteQuantity: 1000.0 * 0.01,
					Fee:           0.01 * 0.05 * 0.01, // 0.05%
					FeeCurrency:   "BTC",
				},
			},
			expectedAverageCost: (1000.0 * 0.01) / (0.01 * (1.0 - feeRate)),
			expectedBase:        0.01 - (0.01 * feeRate),
			expectedQuote:       0 - 1000.0*0.01,
			expectedProfit:      0.0,
		},
		{
			name: "quote fee",
			trades: []Trade{
				{
					Side:          SideTypeSell,
					Price:         1000.0,
					Quantity:      0.01,
					QuoteQuantity: 1000.0 * 0.01,
					Fee:           (1000.0 * 0.01) * feeRate, // 0.05%
					FeeCurrency:   "USDT",
				},
			},
			expectedAverageCost: (1000.0 * 0.01 * (1.0 - feeRate)) / 0.01,
			expectedBase:        -0.01,
			expectedQuote:       0.0 + 1000.0 * 0.01 * (1.0 - feeRate),
			expectedProfit:      0.0,
		},
		{
			name: "long",
			trades: []Trade{
				{
					Side:          SideTypeBuy,
					Price:         1000.0,
					Quantity:      0.01,
					QuoteQuantity: 1000.0 * 0.01,
				},
				{
					Side:          SideTypeBuy,
					Price:         2000.0,
					Quantity:      0.03,
					QuoteQuantity: 2000.0 * 0.03,
				},
			},
			expectedAverageCost: (1000.0*0.01 + 2000.0*0.03) / 0.04,
			expectedBase:        0.01 + 0.03,
			expectedQuote:       0 - 1000.0*0.01 - 2000.0*0.03,
			expectedProfit:      0.0,
		},

		{
			name: "long and sell",
			trades: []Trade{
				{
					Side:          SideTypeBuy,
					Price:         1000.0,
					Quantity:      0.01,
					QuoteQuantity: 1000.0 * 0.01,
				},
				{
					Side:          SideTypeBuy,
					Price:         2000.0,
					Quantity:      0.03,
					QuoteQuantity: 2000.0 * 0.03,
				},
				{
					Side:          SideTypeSell,
					Price:         3000.0,
					Quantity:      0.01,
					QuoteQuantity: 3000.0 * 0.01,
				},
			},
			expectedAverageCost: (1000.0*0.01 + 2000.0*0.03) / 0.04,
			expectedBase:        0.03,
			expectedQuote:       0 - 1000.0*0.01 - 2000.0*0.03 + 3000.0*0.01,
			expectedProfit:      (3000.0 - (1000.0*0.01+2000.0*0.03)/0.04) * 0.01,
		},

		{
			name: "long and sell to short",
			trades: []Trade{
				{
					Side:          SideTypeBuy,
					Price:         1000.0,
					Quantity:      0.01,
					QuoteQuantity: 1000.0 * 0.01,
				},
				{
					Side:          SideTypeBuy,
					Price:         2000.0,
					Quantity:      0.03,
					QuoteQuantity: 2000.0 * 0.03,
				},
				{
					Side:          SideTypeSell,
					Price:         3000.0,
					Quantity:      0.10,
					QuoteQuantity: 3000.0 * 0.10,
				},
			},

			expectedAverageCost: 3000.0,
			expectedBase:        -0.06,
			expectedQuote:       -1000.0*0.01 - 2000.0*0.03 + 3000.0*0.1,
			expectedProfit:      (3000.0 - (1000.0*0.01+2000.0*0.03)/0.04) * 0.04,
		},

		{
			name: "short",
			trades: []Trade{
				{
					Side:          SideTypeSell,
					Price:         2000.0,
					Quantity:      0.01,
					QuoteQuantity: 2000.0 * 0.01,
				},
				{
					Side:          SideTypeSell,
					Price:         3000.0,
					Quantity:      0.03,
					QuoteQuantity: 3000.0 * 0.03,
				},
			},

			expectedAverageCost: (2000.0*0.01 + 3000.0*0.03) / (0.01 + 0.03),
			expectedBase:        0 - 0.01 - 0.03,
			expectedQuote:       2000.0*0.01 + 3000.0*0.03,
			expectedProfit:      0.0,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			pos := Position{
				Symbol:        "BTCUSDT",
				BaseCurrency:  "BTC",
				QuoteCurrency: "USDT",
			}
			profitAmount, _, profit := pos.AddTrades(testcase.trades)
			assert.InDelta(t, testcase.expectedQuote, pos.Quote.Float64(), Delta, "expectedQuote")
			assert.InDelta(t, testcase.expectedBase, pos.Base.Float64(), Delta, "expectedBase")
			assert.InDelta(t, testcase.expectedAverageCost, pos.AverageCost.Float64(), Delta, "expectedAverageCost")
			if profit {
				assert.InDelta(t, testcase.expectedProfit, profitAmount.Float64(), Delta, "expectedProfit")
			}
		})
	}
}
