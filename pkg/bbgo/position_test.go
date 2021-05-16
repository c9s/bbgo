package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestPosition_ExchangeFeeRate_Short(t *testing.T) {
	pos := &Position{
		Symbol:        "BTCUSDT",
		BaseCurrency:  "BTC",
		QuoteCurrency: "USDT",
	}

	feeRate := 0.075 * 0.01
	pos.SetExchangeFeeRate(types.ExchangeBinance, ExchangeFee{
		MakerFeeRate: fixedpoint.NewFromFloat(feeRate),
		TakerFeeRate: fixedpoint.NewFromFloat(feeRate),
	})

	quantity := 10.0
	quoteQuantity := 3000.0 * quantity
	fee := quoteQuantity * feeRate
	averageCost := (quoteQuantity - fee) / quantity
	bnbPrice := 570.0
	pos.AddTrade(types.Trade{
		Exchange:      types.ExchangeBinance,
		Price:         3000.0,
		Quantity:      quantity,
		QuoteQuantity: quoteQuantity,
		Symbol:        "BTCUSDT",
		Side:          types.SideTypeSell,
		Fee:           fee / bnbPrice,
		FeeCurrency:   "BNB",
	})

	profit, madeProfit := pos.AddTrade(types.Trade{
		Exchange:      types.ExchangeBinance,
		Price:         2000.0,
		Quantity:      10.0,
		QuoteQuantity: 2000.0 * 10.0,
		Symbol:        "BTCUSDT",
		Side:          types.SideTypeBuy,
		Fee:           2000.0 * 10.0 * feeRate / bnbPrice,
		FeeCurrency:   "BNB",
	})

	expectedProfit := (averageCost-2000.0)*10.0 - (2000.0 * 10.0 * feeRate)
	assert.True(t, madeProfit)
	assert.Equal(t, fixedpoint.NewFromFloat(expectedProfit), profit)
}

func TestPosition_ExchangeFeeRate_Long(t *testing.T) {
	pos := &Position{
		Symbol:        "BTCUSDT",
		BaseCurrency:  "BTC",
		QuoteCurrency: "USDT",
	}

	feeRate := 0.075 * 0.01
	pos.SetExchangeFeeRate(types.ExchangeBinance, ExchangeFee{
		MakerFeeRate: fixedpoint.NewFromFloat(feeRate),
		TakerFeeRate: fixedpoint.NewFromFloat(feeRate),
	})

	quantity := 10.0
	quoteQuantity := 3000.0 * quantity
	fee := quoteQuantity * feeRate
	averageCost := (quoteQuantity + fee) / quantity
	bnbPrice := 570.0
	pos.AddTrade(types.Trade{
		Exchange:      types.ExchangeBinance,
		Price:         3000.0,
		Quantity:      quantity,
		QuoteQuantity: quoteQuantity,
		Symbol:        "BTCUSDT",
		Side:          types.SideTypeBuy,
		Fee:           fee / bnbPrice,
		FeeCurrency:   "BNB",
	})

	profit, madeProfit := pos.AddTrade(types.Trade{
		Exchange:      types.ExchangeBinance,
		Price:         4000.0,
		Quantity:      10.0,
		QuoteQuantity: 4000.0 * 10.0,
		Symbol:        "BTCUSDT",
		Side:          types.SideTypeSell,
		Fee:           4000.0 * 10.0 * feeRate / bnbPrice,
		FeeCurrency:   "BNB",
	})

	expectedProfit := (4000.0-averageCost)*10.0 - (4000.0 * 10.0 * feeRate)
	assert.True(t, madeProfit)
	assert.Equal(t, fixedpoint.NewFromFloat(expectedProfit), profit)
}

func TestPosition(t *testing.T) {
	var feeRate = 0.05 * 0.01
	var testcases = []struct {
		name                string
		trades              []types.Trade
		expectedAverageCost fixedpoint.Value
		expectedBase        fixedpoint.Value
		expectedQuote       fixedpoint.Value
		expectedProfit      fixedpoint.Value
	}{
		{
			name: "base fee",
			trades: []types.Trade{
				{
					Side:          types.SideTypeBuy,
					Price:         1000.0,
					Quantity:      0.01,
					QuoteQuantity: 1000.0 * 0.01,
					Fee:           0.01 * 0.05 * 0.01, // 0.05%
					FeeCurrency:   "BTC",
				},
			},
			expectedAverageCost: fixedpoint.NewFromFloat((1000.0 * 0.01) / (0.01 * (1.0 - feeRate))),
			expectedBase:        fixedpoint.NewFromFloat(0.01 - (0.01 * feeRate)),
			expectedQuote:       fixedpoint.NewFromFloat(0 - 1000.0*0.01),
			expectedProfit:      fixedpoint.NewFromFloat(0.0),
		},
		{
			name: "quote fee",
			trades: []types.Trade{
				{
					Side:          types.SideTypeSell,
					Price:         1000.0,
					Quantity:      0.01,
					QuoteQuantity: 1000.0 * 0.01,
					Fee:           (1000.0 * 0.01) * feeRate, // 0.05%
					FeeCurrency:   "USDT",
				},
			},
			expectedAverageCost: fixedpoint.NewFromFloat((1000.0 * 0.01 * (1.0 - feeRate)) / 0.01),
			expectedBase:        fixedpoint.NewFromFloat(-0.01),
			expectedQuote:       fixedpoint.NewFromFloat(0 + 1000.0*0.01*(1.0-feeRate)),
			expectedProfit:      fixedpoint.NewFromFloat(0.0),
		},
		{
			name: "long",
			trades: []types.Trade{
				{
					Side:          types.SideTypeBuy,
					Price:         1000.0,
					Quantity:      0.01,
					QuoteQuantity: 1000.0 * 0.01,
				},
				{
					Side:          types.SideTypeBuy,
					Price:         2000.0,
					Quantity:      0.03,
					QuoteQuantity: 2000.0 * 0.03,
				},
			},
			expectedAverageCost: fixedpoint.NewFromFloat((1000.0*0.01 + 2000.0*0.03) / 0.04),
			expectedBase:        fixedpoint.NewFromFloat(0.01 + 0.03),
			expectedQuote:       fixedpoint.NewFromFloat(0 - 1000.0*0.01 - 2000.0*0.03),
			expectedProfit:      fixedpoint.NewFromFloat(0.0),
		},

		{
			name: "long and sell",
			trades: []types.Trade{
				{
					Side:          types.SideTypeBuy,
					Price:         1000.0,
					Quantity:      0.01,
					QuoteQuantity: 1000.0 * 0.01,
				},
				{
					Side:          types.SideTypeBuy,
					Price:         2000.0,
					Quantity:      0.03,
					QuoteQuantity: 2000.0 * 0.03,
				},
				{
					Side:          types.SideTypeSell,
					Price:         3000.0,
					Quantity:      0.01,
					QuoteQuantity: 3000.0 * 0.01,
				},
			},
			expectedAverageCost: fixedpoint.NewFromFloat((1000.0*0.01 + 2000.0*0.03) / 0.04),
			expectedBase:        fixedpoint.NewFromFloat(0.03),
			expectedQuote:       fixedpoint.NewFromFloat(0 - 1000.0*0.01 - 2000.0*0.03 + 3000.0*0.01),
			expectedProfit:      fixedpoint.NewFromFloat((3000.0 - (1000.0*0.01+2000.0*0.03)/0.04) * 0.01),
		},

		{
			name: "long and sell to short",
			trades: []types.Trade{
				{
					Side:          types.SideTypeBuy,
					Price:         1000.0,
					Quantity:      0.01,
					QuoteQuantity: 1000.0 * 0.01,
				},
				{
					Side:          types.SideTypeBuy,
					Price:         2000.0,
					Quantity:      0.03,
					QuoteQuantity: 2000.0 * 0.03,
				},
				{
					Side:          types.SideTypeSell,
					Price:         3000.0,
					Quantity:      0.10,
					QuoteQuantity: 3000.0 * 0.10,
				},
			},

			expectedAverageCost: fixedpoint.NewFromFloat(3000.0),
			expectedBase:        fixedpoint.NewFromFloat(-0.06),
			expectedQuote:       fixedpoint.NewFromFloat(-1000.0*0.01 - 2000.0*0.03 + 3000.0*0.1),
			expectedProfit:      fixedpoint.NewFromFloat((3000.0 - (1000.0*0.01+2000.0*0.03)/0.04) * 0.04),
		},

		{
			name: "short",
			trades: []types.Trade{
				{
					Side:          types.SideTypeSell,
					Price:         2000.0,
					Quantity:      0.01,
					QuoteQuantity: 2000.0 * 0.01,
				},
				{
					Side:          types.SideTypeSell,
					Price:         3000.0,
					Quantity:      0.03,
					QuoteQuantity: 3000.0 * 0.03,
				},
			},

			expectedAverageCost: fixedpoint.NewFromFloat((2000.0*0.01 + 3000.0*0.03) / (0.01 + 0.03)),
			expectedBase:        fixedpoint.NewFromFloat(0 - 0.01 - 0.03),
			expectedQuote:       fixedpoint.NewFromFloat(2000.0*0.01 + 3000.0*0.03),
			expectedProfit:      fixedpoint.NewFromFloat(0.0),
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			pos := Position{
				Symbol:        "BTCUSDT",
				BaseCurrency:  "BTC",
				QuoteCurrency: "USDT",
			}
			profitAmount, profit := pos.AddTrades(testcase.trades)

			assert.Equal(t, testcase.expectedQuote, pos.Quote, "expectedQuote")
			assert.Equal(t, testcase.expectedBase, pos.Base, "expectedBase")
			assert.Equal(t, testcase.expectedAverageCost, pos.AverageCost, "expectedAverageCost")
			if profit {
				assert.Equal(t, testcase.expectedProfit, profitAmount, "expectedProfit")
			}
		})
	}
}
