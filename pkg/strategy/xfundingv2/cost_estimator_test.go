package xfundingv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func newOrderBook(bestBidPrice, bestBidVolume, bestAskPrice, bestAskVolume float64) *types.SliceOrderBook {
	return &types.SliceOrderBook{
		Bids: types.PriceVolumeSlice{
			types.NewPriceVolume(Number(bestBidPrice), Number(bestBidVolume)),
		},
		Asks: types.PriceVolumeSlice{
			types.NewPriceVolume(Number(bestAskPrice), Number(bestAskVolume)),
		},
	}
}

func TestAnnualizedRate(t *testing.T) {
	t.Run("8h funding interval", func(t *testing.T) {
		rate := fixedpoint.NewFromFloat(0.0001)
		annualized := AnnualizedRate(rate, 8)
		// 24*365/8 = 1095
		expected := rate.Mul(fixedpoint.NewFromInt(1095))
		assert.Equal(t, expected, annualized)
	})

	t.Run("4h funding interval", func(t *testing.T) {
		rate := fixedpoint.NewFromFloat(0.0002)
		annualized := AnnualizedRate(rate, 4)
		// 24*365/4 = 2190
		expected := rate.Mul(fixedpoint.NewFromInt(2190))
		assert.Equal(t, expected, annualized)
	})
}

func TestCostEstimator_SettersChaining(t *testing.T) {
	ce := NewCostEstimator().
		SetTargetPosition(Number(-1.0)).
		SetSpotFeeRate(types.ExchangeFee{MakerFeeRate: Number(0.001), TakerFeeRate: Number(0.002)}).
		SetFuturesFeeRate(types.ExchangeFee{MakerFeeRate: Number(0.0005), TakerFeeRate: Number(0.001)})

	assert.Equal(t, Number(-1.0), ce.targetPosition)
	assert.Equal(t, Number(0.001), ce.spotFeeRate.MakerFeeRate)
	assert.Equal(t, Number(0.0005), ce.futuresFeeRate.MakerFeeRate)
}

func TestEstimatedCost_TotalFeeCost(t *testing.T) {
	ec := EstimatedCost{
		SpotFee:    Number(10),
		FuturesFee: Number(5),
	}
	assert.Equal(t, Number(15), ec.TotalFeeCost())
}

func TestEstimatedCost_Spread(t *testing.T) {
	ec := EstimatedCost{
		SpreadPnL:       Number(100),
		FuturesPosition: Number(2),
	}
	assert.Equal(t, Number(50), ec.Spread())
}

func TestCostEstimator_EstimateEntryCost(t *testing.T) {
	spotFee := types.ExchangeFee{MakerFeeRate: Number(0.001), TakerFeeRate: Number(0.002)}
	futuresFee := types.ExchangeFee{MakerFeeRate: Number(0.0005), TakerFeeRate: Number(0.001)}

	t.Run("zero position returns zero cost", func(t *testing.T) {
		ce := NewCostEstimator().SetTargetPosition(Number(0))
		spotOB := newOrderBook(100, 10, 101, 10)
		futuresOB := newOrderBook(100, 10, 101, 10)
		cost, err := ce.EstimateEntryCost(false, spotOB, futuresOB)
		assert.NoError(t, err)
		assert.True(t, cost.SpotFee.IsZero())
		assert.True(t, cost.FuturesFee.IsZero())
		assert.True(t, cost.SpreadPnL.IsZero())
	})

	t.Run("short futures entry (negative target)", func(t *testing.T) {
		// short futures, long spot: buy spot at ask, sell futures at bid
		ce := NewCostEstimator().
			SetTargetPosition(Number(-1.0)).
			SetSpotFeeRate(spotFee).
			SetFuturesFeeRate(futuresFee)

		spotOB := newOrderBook(99, 10, 100, 10)     // best ask = 100
		futuresOB := newOrderBook(101, 10, 102, 10) // best bid = 101

		cost, err := ce.EstimateEntryCost(false, spotOB, futuresOB)
		assert.NoError(t, err)

		// spread = spot - futures = 100 - 101 = -1
		// spreadPnL = spread * targetPosition = -1 * -1 = 1
		assert.Equal(t, Number(1), cost.SpreadPnL)

		// positionSize = abs(-1) = 1
		// spotFee = 100 * 1 * 0.002 = 0.2
		assert.Equal(t, Number(0.2), cost.SpotFee)
		// futuresFee = 101 * 1 * 0.001 = 0.101
		assert.Equal(t, Number(0.101), cost.FuturesFee)
		assert.Equal(t, Number(-1), cost.FuturesPosition)
	})

	t.Run("long futures entry (positive target)", func(t *testing.T) {
		// long futures, short spot: sell spot at bid, buy futures at ask
		ce := NewCostEstimator().
			SetTargetPosition(Number(2.0)).
			SetSpotFeeRate(spotFee).
			SetFuturesFeeRate(futuresFee)

		spotOB := newOrderBook(99, 10, 100, 10)     // best bid = 99
		futuresOB := newOrderBook(101, 10, 102, 10) // best ask = 102

		cost, err := ce.EstimateEntryCost(false, spotOB, futuresOB)
		assert.NoError(t, err)

		// spread = spot - futures = 99 - 102 = -3
		// spreadPnL = -3 * 2 = -6
		assert.Equal(t, Number(-6), cost.SpreadPnL)

		// positionSize = 2
		// spotFee = 99 * 2 * 0.002 = 0.396
		assert.Equal(t, Number(0.396), cost.SpotFee)
		// futuresFee = 102 * 2 * 0.001 = 0.204
		assert.Equal(t, Number(0.204), cost.FuturesFee)
	})

	t.Run("maker fee rates", func(t *testing.T) {
		ce := NewCostEstimator().
			SetTargetPosition(Number(-1.0)).
			SetSpotFeeRate(spotFee).
			SetFuturesFeeRate(futuresFee)

		spotOB := newOrderBook(99, 10, 100, 10)
		futuresOB := newOrderBook(101, 10, 102, 10)

		cost, err := ce.EstimateEntryCost(true, spotOB, futuresOB)
		assert.NoError(t, err)

		// maker: spotFee = 100 * 1 * 0.001 = 0.1
		assert.Equal(t, Number(0.1), cost.SpotFee)
		// maker: futuresFee = 101 * 1 * 0.0005 = 0.0505
		assert.Equal(t, Number("0.0505"), cost.FuturesFee)
	})

	t.Run("empty order book returns error", func(t *testing.T) {
		ce := NewCostEstimator().
			SetTargetPosition(Number(-1.0)).
			SetSpotFeeRate(spotFee).
			SetFuturesFeeRate(futuresFee)

		emptyOB := &types.SliceOrderBook{}
		spotOB := newOrderBook(100, 10, 101, 10)

		_, err := ce.EstimateEntryCost(false, emptyOB, spotOB)
		assert.Error(t, err)

		_, err = ce.EstimateEntryCost(false, spotOB, emptyOB)
		assert.Error(t, err)
	})
}

func TestCostEstimator_EstimateExitCost(t *testing.T) {
	spotFee := types.ExchangeFee{MakerFeeRate: Number(0.001), TakerFeeRate: Number(0.002)}
	futuresFee := types.ExchangeFee{MakerFeeRate: Number(0.0005), TakerFeeRate: Number(0.001)}

	t.Run("zero position returns zero cost", func(t *testing.T) {
		ce := NewCostEstimator().SetTargetPosition(Number(0))
		spotOB := newOrderBook(100, 10, 101, 10)
		futuresOB := newOrderBook(100, 10, 101, 10)
		cost, err := ce.EstimateExitCost(false, spotOB, futuresOB)
		assert.NoError(t, err)
		assert.True(t, cost.SpotFee.IsZero())
	})

	t.Run("exit short futures (negative target)", func(t *testing.T) {
		// to close short futures: buy futures at ask, sell spot at bid
		ce := NewCostEstimator().
			SetTargetPosition(Number(-1.0)).
			SetSpotFeeRate(spotFee).
			SetFuturesFeeRate(futuresFee)

		spotOB := newOrderBook(99, 10, 100, 10)     // best bid = 99
		futuresOB := newOrderBook(101, 10, 102, 10) // best ask = 102

		cost, err := ce.EstimateExitCost(false, spotOB, futuresOB)
		assert.NoError(t, err)

		// spread = spot - futures = 99 - 102 = -3
		// spreadPnL = -3 * -1 = 3
		assert.Equal(t, Number(3), cost.SpreadPnL)

		// spotFee = 99 * 1 * 0.002 = 0.198
		assert.Equal(t, Number(0.198), cost.SpotFee)
		// futuresFee = 102 * 1 * 0.001 = 0.102
		assert.Equal(t, Number(0.102), cost.FuturesFee)
	})

	t.Run("exit long futures (positive target)", func(t *testing.T) {
		// to close long futures: sell futures at bid, buy spot at ask
		ce := NewCostEstimator().
			SetTargetPosition(Number(2.0)).
			SetSpotFeeRate(spotFee).
			SetFuturesFeeRate(futuresFee)

		spotOB := newOrderBook(99, 10, 100, 10)     // best ask = 100
		futuresOB := newOrderBook(101, 10, 102, 10) // best bid = 101

		cost, err := ce.EstimateExitCost(false, spotOB, futuresOB)
		assert.NoError(t, err)

		// spread = spot - futures = 100 - 101 = -1
		// spreadPnL = -1 * 2 = -2
		assert.Equal(t, Number(-2), cost.SpreadPnL)

		// spotFee = 100 * 2 * 0.002 = 0.4
		assert.Equal(t, Number(0.4), cost.SpotFee)
		// futuresFee = 101 * 2 * 0.001 = 0.202
		assert.Equal(t, Number(0.202), cost.FuturesFee)
	})

	t.Run("empty order book returns error", func(t *testing.T) {
		ce := NewCostEstimator().
			SetTargetPosition(Number(-1.0)).
			SetSpotFeeRate(spotFee).
			SetFuturesFeeRate(futuresFee)

		emptyOB := &types.SliceOrderBook{}
		spotOB := newOrderBook(100, 10, 101, 10)

		_, err := ce.EstimateExitCost(false, emptyOB, spotOB)
		assert.Error(t, err)

		_, err = ce.EstimateExitCost(false, spotOB, emptyOB)
		assert.Error(t, err)
	})
}
