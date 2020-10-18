package accounting

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestStockManager(t *testing.T) {

	t.Run("testdata", func(t *testing.T) {
		var trades []types.Trade

		out, err := ioutil.ReadFile("testdata/btcusdt-trades.json")
		assert.NoError(t, err)

		err = json.Unmarshal(out, &trades)
		assert.NoError(t, err)

		var stockManager = &StockDistribution{
			TradingFeeCurrency: "BNB",
			Symbol:             "BTCUSDT",
		}

		_, err = stockManager.AddTrades(trades)
		assert.NoError(t, err)
		assert.Equal(t, 0.72970242, stockManager.Stocks.Quantity())
		assert.NotEmpty(t, stockManager.Stocks)
		assert.Equal(t, 20, len(stockManager.Stocks))
		assert.Equal(t, 0, len(stockManager.PendingSells))

	})

	t.Run("stock", func(t *testing.T) {
		var trades = []types.Trade{
			{Symbol: "BTCUSDT", Price: 9100.0, Quantity: 0.05, IsBuyer: true},
			{Symbol: "BTCUSDT", Price: 9100.0, Quantity: 0.05, IsBuyer: true},
			{Symbol: "BTCUSDT", Price: 9200.0, Quantity: 0.01, IsBuyer: false},
		}

		var stockManager = &StockDistribution{
			TradingFeeCurrency: "BNB",
			Symbol:             "BTCUSDT",
		}

		_, err := stockManager.AddTrades(trades)
		assert.NoError(t, err)
		assert.Len(t, stockManager.Stocks, 2)
		assert.Equal(t, StockSlice{
			{
				Symbol:   "BTCUSDT",
				Price:    9100.0,
				Quantity: 0.05,
				IsBuyer:  true,
			},
			{
				Symbol:   "BTCUSDT",
				Price:    9100.0,
				Quantity: 0.04,
				IsBuyer:  true,
			},
		}, stockManager.Stocks)
		assert.Len(t, stockManager.PendingSells, 0)
	})

	t.Run("sold out", func(t *testing.T) {
		var trades = []types.Trade{
			{Symbol: "BTCUSDT", Price: 9100.0, Quantity: 0.05, IsBuyer: true},
			{Symbol: "BTCUSDT", Price: 9200.0, Quantity: 0.05, IsBuyer: false},
			{Symbol: "BTCUSDT", Price: 9100.0, Quantity: 0.05, IsBuyer: true},
			{Symbol: "BTCUSDT", Price: 9200.0, Quantity: 0.05, IsBuyer: false},
		}

		var stockManager = &StockDistribution{
			TradingFeeCurrency: "BNB",
			Symbol:             "BTCUSDT",
		}

		_, err := stockManager.AddTrades(trades)
		assert.NoError(t, err)
		assert.Len(t, stockManager.Stocks, 0)
		assert.Len(t, stockManager.PendingSells, 0)
	})

	t.Run("oversell", func(t *testing.T) {
		var trades = []types.Trade{
			{Symbol: "BTCUSDT", Price: 9100.0, Quantity: 0.05, IsBuyer: true},
			{Symbol: "BTCUSDT", Price: 9200.0, Quantity: 0.05, IsBuyer: false},
			{Symbol: "BTCUSDT", Price: 9200.0, Quantity: 0.05, IsBuyer: false},
		}

		var stockManager = &StockDistribution{
			TradingFeeCurrency: "BNB",
			Symbol:             "BTCUSDT",
		}

		_, err := stockManager.AddTrades(trades)
		assert.NoError(t, err)
		assert.Len(t, stockManager.Stocks, 0)
		assert.Len(t, stockManager.PendingSells, 1)
	})

	t.Run("loss sell", func(t *testing.T) {
		var trades = []types.Trade{
			{Symbol: "BTCUSDT", Price: 9100.0, Quantity: 0.05, IsBuyer: true},
			{Symbol: "BTCUSDT", Price: 9200.0, Quantity: 0.02, IsBuyer: false},
			{Symbol: "BTCUSDT", Price: 8000.0, Quantity: 0.01, IsBuyer: false},
		}

		var stockManager = &StockDistribution{
			TradingFeeCurrency: "BNB",
			Symbol:             "BTCUSDT",
		}

		_, err := stockManager.AddTrades(trades)
		assert.NoError(t, err)
		assert.Len(t, stockManager.Stocks, 1)
		assert.Equal(t, StockSlice{
			{
				Symbol:   "BTCUSDT",
				Price:    9100.0,
				Quantity: 0.02,
				IsBuyer:  true,
			},
		}, stockManager.Stocks)
		assert.Len(t, stockManager.PendingSells, 0)
	})

	t.Run("pending sell 1", func(t *testing.T) {
		var trades = []types.Trade{
			{Symbol: "BTCUSDT", Price: 9200.0, Quantity: 0.02},
			{Symbol: "BTCUSDT", Price: 9100.0, Quantity: 0.05, IsBuyer: true},
		}

		var stockManager = &StockDistribution{
			TradingFeeCurrency: "BNB",
			Symbol:             "BTCUSDT",
		}

		_, err := stockManager.AddTrades(trades)
		assert.NoError(t, err)
		assert.Len(t, stockManager.Stocks, 1)
		assert.Equal(t, StockSlice{
			{
				Symbol:   "BTCUSDT",
				Price:    9100.0,
				Quantity: 0.03,
				IsBuyer:  true,
			},
		}, stockManager.Stocks)
		assert.Len(t, stockManager.PendingSells, 0)
	})

	t.Run("pending sell 2", func(t *testing.T) {
		var trades = []types.Trade{
			{Symbol: "BTCUSDT", Price: 9200.0, Quantity: 0.1},
			{Symbol: "BTCUSDT", Price: 9100.0, Quantity: 0.05, IsBuyer: true},
		}

		var stockManager = &StockDistribution{
			TradingFeeCurrency: "BNB",
			Symbol:             "BTCUSDT",
		}

		_, err := stockManager.AddTrades(trades)
		assert.NoError(t, err)
		assert.Len(t, stockManager.Stocks, 0)
		assert.Len(t, stockManager.PendingSells, 1)
		assert.Equal(t, StockSlice{
			{
				Symbol:   "BTCUSDT",
				Price:    9200.0,
				Quantity: 0.05,
				IsBuyer:  false,
			},
		}, stockManager.PendingSells)
	})

}
