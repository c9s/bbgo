package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestLiquidityDemand(t *testing.T) {
	t.Run("Buy-Side Demand", func(t *testing.T) {
		liqDemand := newLiquidityDemandStream()
		// Test case where price range above open is small (high demand concentration)
		// and price range below open is large (low sell pressure)
		for _, kline := range []types.KLine{
			{
				Open:   fixedpoint.NewFromFloat(100),
				Close:  fixedpoint.NewFromFloat(105),
				High:   fixedpoint.NewFromFloat(102), // Small range above open: 2
				Low:    fixedpoint.NewFromFloat(80),  // Large range below open: 20
				Volume: fixedpoint.NewFromFloat(1000),
			},
			{
				Open:   fixedpoint.NewFromFloat(100),
				Close:  fixedpoint.NewFromFloat(110),
				High:   fixedpoint.NewFromFloat(103), // Small range above open: 3
				Low:    fixedpoint.NewFromFloat(75),  // Large range below open: 25
				Volume: fixedpoint.NewFromFloat(1500),
			},
		} {
			liqDemand.handleKLine(kline)
		}
		netDemand := liqDemand.Last(0)
		assert.True(t, netDemand > 0, "Expected positive net demand for buy-side scenario, got %f", netDemand)
	})

	t.Run("Sell-Side Demand", func(t *testing.T) {
		liqDemand := newLiquidityDemandStream()
		// Test case where price range below open is small (high sell pressure)
		// and price range above open is large (low buy demand)
		for _, kline := range []types.KLine{
			{
				Open:   fixedpoint.NewFromFloat(100),
				Close:  fixedpoint.NewFromFloat(95),
				High:   fixedpoint.NewFromFloat(120), // Large range above open: 20
				Low:    fixedpoint.NewFromFloat(98),  // Small range below open: 2
				Volume: fixedpoint.NewFromFloat(1000),
			},
			{
				Open:   fixedpoint.NewFromFloat(100),
				Close:  fixedpoint.NewFromFloat(90),
				High:   fixedpoint.NewFromFloat(125), // Large range above open: 25
				Low:    fixedpoint.NewFromFloat(97),  // Small range below open: 3
				Volume: fixedpoint.NewFromFloat(1500),
			},
		} {
			liqDemand.handleKLine(kline)
		}
		netDemand := liqDemand.Last(0)
		assert.True(t, netDemand < 0, "Expected negative net demand for sell-side scenario, got %f", netDemand)
	})

	t.Run("Constructor Function", func(t *testing.T) {
		klineStream := &KLineStream{}
		sellMA := &SMAStream{
			Float64Series: types.NewFloat64Series(),
			window:        3,
			rawValues:     types.NewQueue(10),
		}
		buyMA := &SMAStream{
			Float64Series: types.NewFloat64Series(),
			window:        3,
			rawValues:     types.NewQueue(10),
		}

		liqDemand := LiquidityDemand(klineStream, sellMA, buyMA)

		assert.NotNil(t, liqDemand)
		assert.Equal(t, sellMA, liqDemand.sellDemandMA)
		assert.Equal(t, buyMA, liqDemand.buyDemandMA)
		assert.Equal(t, klineStream, liqDemand.kLineStream)
		assert.Equal(t, fixedpoint.NewFromFloat(1e-5), liqDemand.epsilon)
	})
}

func Test_EdgeCases(t *testing.T) {
	t.Run("Zero Price Range Protection", func(t *testing.T) {
		liqDemand := newLiquidityDemandStream()
		// Test case where open equals high (no upward price movement)
		kline := types.KLine{
			Open:   fixedpoint.NewFromFloat(100),
			Close:  fixedpoint.NewFromFloat(100),
			High:   fixedpoint.NewFromFloat(100), // Same as open
			Low:    fixedpoint.NewFromFloat(95),
			Volume: fixedpoint.NewFromFloat(1000),
		}
		liqDemand.handleKLine(kline)
		netDemand := liqDemand.Last(0)
		assert.NotEqual(t, 0.0, netDemand, "Net demand should not be zero when using epsilon")
	})

	t.Run("Equal Buy and Sell Demand", func(t *testing.T) {
		liqDemand := newLiquidityDemandStream()
		// Test case where buy and sell pressure are roughly equal
		for _, kline := range []types.KLine{
			{
				Open:   fixedpoint.NewFromFloat(100),
				Close:  fixedpoint.NewFromFloat(100),
				High:   fixedpoint.NewFromFloat(110), // Range above: 10
				Low:    fixedpoint.NewFromFloat(90),  // Range below: 10
				Volume: fixedpoint.NewFromFloat(1000),
			},
			{
				Open:   fixedpoint.NewFromFloat(100),
				Close:  fixedpoint.NewFromFloat(100),
				High:   fixedpoint.NewFromFloat(115), // Range above: 15
				Low:    fixedpoint.NewFromFloat(85),  // Range below: 15
				Volume: fixedpoint.NewFromFloat(1500),
			},
		} {
			liqDemand.handleKLine(kline)
		}
		netDemand := liqDemand.Last(0)
		// With equal ranges, net demand should be close to zero
		assert.InDelta(t, 0.0, netDemand, 0.1, "Net demand should be close to zero for equal buy/sell pressure")
	})

	t.Run("Both Price Ranges Below Epsilon", func(t *testing.T) {
		liqDemand := newLiquidityDemandStream()
		// Test case where both price ranges are extremely small (below epsilon)
		// This simulates a candle with almost no price movement
		kline := types.KLine{
			Open:   fixedpoint.NewFromFloat(100.0),
			Close:  fixedpoint.NewFromFloat(100.0),
			High:   fixedpoint.NewFromFloat(100.0000001), // Tiny movement up (< 1e-6)
			Low:    fixedpoint.NewFromFloat(99.9999999),  // Tiny movement down (< 1e-6)
			Volume: fixedpoint.NewFromFloat(1000000.0),   // Large volume
		}
		liqDemand.handleKLine(kline)
		netDemand := liqDemand.Last(0)
		// Should return 0 to indicate no clear directional signal
		assert.Equal(t, 0.0, netDemand, "Net demand should be 0 when both price ranges are below epsilon")
	})
}

func newLiquidityDemandStream() *LiquidityDemandStream {
	return &LiquidityDemandStream{
		Float64Series: types.NewFloat64Series(),
		sellDemandMA: &SMAStream{
			Float64Series: types.NewFloat64Series(),
			window:        2,
			rawValues:     types.NewQueue(10),
		},
		buyDemandMA: &SMAStream{
			Float64Series: types.NewFloat64Series(),
			window:        2,
			rawValues:     types.NewQueue(10),
		},
		kLineStream: nil,
		epsilon:     fixedpoint.NewFromFloat(1e-6),
	}
}
