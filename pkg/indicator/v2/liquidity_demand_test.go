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
		for _, kline := range []types.KLine{
			{
				Open:   fixedpoint.NewFromFloat(10),
				Close:  fixedpoint.NewFromFloat(20),
				High:   fixedpoint.NewFromFloat(25),
				Low:    fixedpoint.NewFromFloat(5),
				Volume: fixedpoint.NewFromFloat(100),
			},
			{
				Open:   fixedpoint.NewFromFloat(20),
				Close:  fixedpoint.NewFromFloat(30),
				High:   fixedpoint.NewFromFloat(35),
				Low:    fixedpoint.NewFromFloat(15),
				Volume: fixedpoint.NewFromFloat(30),
			},
		} {
			liqDemand.handleKLine(kline)
		}
		netDemand := liqDemand.Last(0)
		assert.True(t, netDemand > 0)
	})

	t.Run("Sell-Side Demand", func(t *testing.T) {
		liqDemand := newLiquidityDemandStream()
		for _, kline := range []types.KLine{
			{
				Open:   fixedpoint.NewFromFloat(20),
				Close:  fixedpoint.NewFromFloat(10),
				High:   fixedpoint.NewFromFloat(25),
				Low:    fixedpoint.NewFromFloat(5),
				Volume: fixedpoint.NewFromFloat(100),
			},
			{
				Open:   fixedpoint.NewFromFloat(30),
				Close:  fixedpoint.NewFromFloat(20),
				High:   fixedpoint.NewFromFloat(35),
				Low:    fixedpoint.NewFromFloat(15),
				Volume: fixedpoint.NewFromFloat(30),
			},
		} {
			liqDemand.handleKLine(kline)
		}
		netDemand := liqDemand.Last(0)
		assert.True(t, netDemand < 0)
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
