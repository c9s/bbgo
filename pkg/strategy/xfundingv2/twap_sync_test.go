package xfundingv2

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func TestTWAPWorker_MarshalJSON(t *testing.T) {
	t.Run("all fields present", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)
		w := &TWAPWorker{
			syncState: TWAPWorkerSyncState{
				Config: TWAPWorkerConfig{
					Duration:    types.Duration(10 * time.Minute),
					NumSlices:   5,
					OrderType:   TWAPOrderTypeTaker,
					MaxSlippage: fixedpoint.NewFromFloat(0.001),
				},
				TargetPosition:       fixedpoint.NewFromFloat(1.5),
				State:                TWAPWorkerStateRunning,
				StartAt:              now,
				EndAt:                now.Add(10 * time.Minute),
				PlaceOrderInterval:   2 * time.Minute,
				CurrentIntervalStart: now,
				CurrentIntervalEnd:   now.Add(2 * time.Minute),
				LastCheckTime:        now.Add(1 * time.Minute),
				Symbol:               "BTCUSDT",
			},
		}

		data, err := json.Marshal(w)
		require.NoError(t, err)

		var raw map[string]json.RawMessage
		err = json.Unmarshal(data, &raw)
		require.NoError(t, err)

		assert.Contains(t, raw, "config")
		assert.Contains(t, raw, "symbol")
	})

	t.Run("nil active order omitted", func(t *testing.T) {
		w := &TWAPWorker{
			syncState: TWAPWorkerSyncState{
				Config: TWAPWorkerConfig{
					Duration:  types.Duration(5 * time.Minute),
					NumSlices: 3,
				},
				TargetPosition: fixedpoint.NewFromFloat(2.0),
				State:          TWAPWorkerStatePending,
				Symbol:         "ETHUSDT",
			},
		}

		data, err := json.Marshal(w)
		require.NoError(t, err)

		var raw map[string]json.RawMessage
		err = json.Unmarshal(data, &raw)
		require.NoError(t, err)

		assert.NotContains(t, raw, "activeOrder")
		assert.Contains(t, raw, "symbol")
	})
}

func TestTWAPWorker_UnmarshalJSON(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)

		market := Market("BTCUSDT")
		market.Exchange = types.ExchangeBinance

		executor := &TWAPExecutor{
			syncState: TWAPExecutorSyncState{
				Config: TWAPWorkerConfig{
					Duration:  types.Duration(10 * time.Minute),
					NumSlices: 5,
					OrderType: TWAPOrderTypeTaker,
				},
				Market:    market,
				IsFutures: true,
				Orders:    map[uint64]types.OrderQuery{},
				Trades:    map[uint64]types.Trade{},
			},
		}

		original := &TWAPWorker{
			syncState: TWAPWorkerSyncState{
				Config: TWAPWorkerConfig{
					Duration:      types.Duration(10 * time.Minute),
					NumSlices:     5,
					OrderType:     TWAPOrderTypeTaker,
					MaxSlippage:   fixedpoint.NewFromFloat(0.001),
					CheckInterval: types.Duration(30 * time.Second),
				},
				TargetPosition:       fixedpoint.NewFromFloat(1.5),
				State:                TWAPWorkerStateRunning,
				StartAt:              now,
				EndAt:                now.Add(10 * time.Minute),
				PlaceOrderInterval:   2 * time.Minute,
				CurrentIntervalStart: now,
				CurrentIntervalEnd:   now.Add(2 * time.Minute),
				LastCheckTime:        now.Add(1 * time.Minute),
				Symbol:               "BTCUSDT",
				TWAPExecutor:         executor,
			},
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		restored := &TWAPWorker{}
		err = json.Unmarshal(data, restored)
		require.NoError(t, err)

		assert.Equal(t, original.syncState.Config, restored.syncState.Config)
		assert.Equal(t, original.syncState.TargetPosition, restored.syncState.TargetPosition)
		assert.Equal(t, original.syncState.State, restored.syncState.State)
		assert.Equal(t, original.syncState.Symbol, restored.syncState.Symbol)
		require.NotNil(t, restored.syncState.TWAPExecutor)
		assert.True(t, restored.syncState.TWAPExecutor.syncState.IsFutures)
	})

	t.Run("nil active order", func(t *testing.T) {
		original := &TWAPWorker{
			syncState: TWAPWorkerSyncState{
				Config: TWAPWorkerConfig{
					Duration:  types.Duration(5 * time.Minute),
					NumSlices: 3,
				},
				TargetPosition: fixedpoint.NewFromFloat(2.0),
				State:          TWAPWorkerStateDone,
				Symbol:         "ETHUSDT",
			},
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		restored := &TWAPWorker{}
		err = json.Unmarshal(data, restored)
		require.NoError(t, err)

		assert.Equal(t, original.syncState.Config, restored.syncState.Config)
		assert.Equal(t, original.syncState.TargetPosition, restored.syncState.TargetPosition)
		assert.Equal(t, original.syncState.State, restored.syncState.State)
		assert.Equal(t, original.syncState.Symbol, restored.syncState.Symbol)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		w := &TWAPWorker{}
		err := w.UnmarshalJSON([]byte(`{invalid json`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal TWAPWorker")
	})
}
