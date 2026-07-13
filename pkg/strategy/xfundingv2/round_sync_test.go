package xfundingv2

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func TestArbitrageRound_MarshalUnmarshalJSON(t *testing.T) {
	t.Run("round_trip_preserves_all_fields", func(t *testing.T) {
		startTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		closingTime := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
		lastUpdateTime := time.Date(2025, 1, 2, 1, 0, 0, 0, time.UTC)
		fundingStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		fundingEnd := time.Date(2025, 1, 1, 7, 59, 59, 0, time.UTC)

		round := &ArbitrageRound{
			syncState: ArbitrageRoundSyncState{
				TriggeredFundingRate:        fixedpoint.NewFromFloat(0.001),
				TriggeredSpotTargetPosition: fixedpoint.NewFromFloat(0.5),
				MinHoldingIntervals:         3,
				FundingIntervalHours:        8,
				FundingIntervalStart:        fundingStart,
				FundingIntervalEnd:          fundingEnd,
				FundingFeeRecords: map[int64]FundingFee{
					100: {
						Asset:  "BTC",
						Amount: fixedpoint.NewFromFloat(0.0001),
						Txn:    100,
						Time:   startTime,
					},
				},
				Symbol:              "BTCUSDT",
				SpotExchangeName:    types.ExchangeBinance,
				FuturesExchangeName: types.ExchangeBinance,
				DirectionPolicy: directionPolicy{
					Direction: types.PositionShort,
					Market:    Market("BTCUSDT"),
				},

				SpotFeeAssetAmount:    fixedpoint.NewFromFloat(0.01),
				FuturesFeeAssetAmount: fixedpoint.NewFromFloat(0.02),
				FeeSymbol:             "BNB",
				AvgFeeCost:            fixedpoint.NewFromFloat(600.0),

				RetryDuration: 5 * time.Minute,
				RetryTransfers: map[uint64]*transferRetry{
					1: {
						Trade: types.Trade{
							ID:       1,
							Exchange: types.ExchangeBinance,
							Side:     types.SideTypeBuy,
						},
						LastTried: startTime,
					},
				},

				State:           RoundReady,
				StartAt:         startTime,
				ClosingAt:       closingTime,
				ClosingDuration: types.Duration(30 * time.Minute),
				LastUpdateTime:  lastUpdateTime,
			},
		}

		data, err := json.Marshal(round)
		require.NoError(t, err)

		var restored ArbitrageRound
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.Equal(t, round.syncState, restored.syncState)
	})

	t.Run("nil_workers_are_preserved", func(t *testing.T) {
		round := &ArbitrageRound{
			syncState: ArbitrageRoundSyncState{
				TriggeredFundingRate: fixedpoint.NewFromFloat(0.0005),
				SpotExchangeName:     types.ExchangeBinance,
				FuturesExchangeName:  types.ExchangeBinance,
				State:                RoundPending,
				DirectionPolicy: directionPolicy{
					Direction: types.PositionShort,
					Market:    Market("ETHUSDT"),
				},
				FundingFeeRecords: make(map[int64]FundingFee),
			},
		}

		data, err := json.Marshal(round)
		require.NoError(t, err)

		var restored ArbitrageRound
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.Nil(t, restored.spotWorker)
		assert.Nil(t, restored.futuresWorker)
		assert.Equal(t, round.syncState, restored.syncState)
	})

	t.Run("unmarshal_invalid_json_returns_error", func(t *testing.T) {
		var round ArbitrageRound
		err := json.Unmarshal([]byte(`{invalid`), &round)
		assert.Error(t, err)
	})
}
