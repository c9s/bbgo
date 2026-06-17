package xfundingv2

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func TestTWAPExecutor_MarshalJSON(t *testing.T) {
	market := Market("BTCUSDT")
	market.Exchange = types.ExchangeBinance

	config := TWAPWorkerConfig{
		Duration:    types.Duration(5 * time.Minute),
		NumSlices:   10,
		OrderType:   TWAPOrderTypeMaker,
		NumOfTicks:  3,
		MaxSlippage: Number(0.001),
	}

	executor := &TWAPExecutor{
		syncState: TWAPExecutorSyncState{
			Config:    config,
			Market:    market,
			IsFutures: true,
			Orders: map[uint64]types.OrderQuery{
				123: {Symbol: "BTCUSDT", OrderID: "123"},
			},
			Trades: map[uint64]types.Trade{},
		},
	}

	data, err := json.Marshal(executor)
	assert.NoError(t, err)

	// Verify JSON structure has expected top-level keys
	var raw map[string]json.RawMessage
	err = json.Unmarshal(data, &raw)
	assert.NoError(t, err)

	assert.Contains(t, raw, "config")
	assert.Contains(t, raw, "isFutures")
	assert.Contains(t, raw, "market")

	// Verify isFutures=true and orders at top level
	var stateData TWAPExecutorSyncState
	err = json.Unmarshal(data, &stateData)
	assert.NoError(t, err)
	assert.True(t, stateData.IsFutures)
	assert.Len(t, stateData.Orders, 1)

	// Round-trip: unmarshal back and verify fields
	var restored TWAPExecutor
	err = json.Unmarshal(data, &restored)
	assert.NoError(t, err)

	assert.Equal(t, config, restored.syncState.Config)
	assert.Equal(t, market.Symbol, restored.syncState.Market.Symbol)
	assert.Equal(t, types.ExchangeBinance, restored.syncState.Market.Exchange)
	assert.True(t, restored.syncState.IsFutures)
	assert.Len(t, restored.syncState.Orders, 1)
	assert.Equal(t, "123", restored.syncState.Orders[123].OrderID)
}

func TestTWAPExecutor_MarshalJSON_NotFutures(t *testing.T) {
	market := Market("BTCUSDT")
	market.Exchange = types.ExchangeBinance

	executor := &TWAPExecutor{
		syncState: TWAPExecutorSyncState{
			Config: TWAPWorkerConfig{
				OrderType: TWAPOrderTypeTaker,
			},
			Market:    market,
			IsFutures: false,
			Orders:    map[uint64]types.OrderQuery{},
			Trades:    map[uint64]types.Trade{},
		},
	}

	data, err := json.Marshal(executor)
	assert.NoError(t, err)

	var restored TWAPExecutor
	err = json.Unmarshal(data, &restored)
	assert.NoError(t, err)

	assert.False(t, restored.syncState.IsFutures)
	assert.Equal(t, TWAPOrderTypeTaker, restored.syncState.Config.OrderType)
}

func TestTWAPExecutor_UnmarshalJSON(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		// types.Duration marshals as a string (e.g. "5m0s")
		jsonData := `{
			"config": {
				"duration": "5m",
				"numSlices": 10,
				"orderType": "maker",
				"numOfTicks": 3
			},
			"market": {"symbol": "BTCUSDT", "exchange": "binance"},
			"isFutures": false,
			"orders": {
				"100": {"symbol": "BTCUSDT", "orderID": "100"}
			},
			"trades": {}
		}`

		var executor TWAPExecutor
		err := json.Unmarshal([]byte(jsonData), &executor)
		assert.NoError(t, err)
		assert.Equal(t, types.Duration(5*time.Minute), executor.syncState.Config.Duration)
		assert.Equal(t, 10, executor.syncState.Config.NumSlices)
		assert.Equal(t, TWAPOrderTypeMaker, executor.syncState.Config.OrderType)
		assert.Equal(t, "BTCUSDT", executor.syncState.Market.Symbol)
		assert.False(t, executor.syncState.IsFutures)
		assert.Len(t, executor.syncState.Orders, 1)
	})

	t.Run("futures flag", func(t *testing.T) {
		jsonData := `{
			"config": {},
			"market": {"symbol": "ETHUSDT", "exchange": "binance"},
			"isFutures": true,
			"orders": {},
			"trades": {}
		}`

		var executor TWAPExecutor
		err := json.Unmarshal([]byte(jsonData), &executor)
		assert.NoError(t, err)
		assert.True(t, executor.syncState.IsFutures)
		assert.Equal(t, "ETHUSDT", executor.syncState.Market.Symbol)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		var executor TWAPExecutor
		err := json.Unmarshal([]byte(`{invalid`), &executor)
		assert.Error(t, err)
	})
}
