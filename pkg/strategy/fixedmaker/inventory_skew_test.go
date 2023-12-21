package fixedmaker

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
)

func Test_InventorySkew_CalculateBidAskRatios(t *testing.T) {
	cases := []struct {
		quantity     fixedpoint.Value
		price        fixedpoint.Value
		baseBalance  fixedpoint.Value
		quoteBalance fixedpoint.Value
		want         *InventorySkewBidAskRatios
	}{
		{
			quantity:     fixedpoint.NewFromFloat(1.0),
			price:        fixedpoint.NewFromFloat(1000),
			baseBalance:  fixedpoint.NewFromFloat(1.0),
			quoteBalance: fixedpoint.NewFromFloat(1000),
			want: &InventorySkewBidAskRatios{
				BidRatio: fixedpoint.NewFromFloat(1.0),
				AskRatio: fixedpoint.NewFromFloat(1.0),
			},
		},
		{
			quantity:     fixedpoint.NewFromFloat(1.0),
			price:        fixedpoint.NewFromFloat(1000),
			baseBalance:  fixedpoint.NewFromFloat(1.0),
			quoteBalance: fixedpoint.NewFromFloat(1200),
			want: &InventorySkewBidAskRatios{
				BidRatio: fixedpoint.NewFromFloat(1.5),
				AskRatio: fixedpoint.NewFromFloat(0.5),
			},
		},
		{
			quantity:     fixedpoint.NewFromFloat(1.0),
			price:        fixedpoint.NewFromFloat(1000),
			baseBalance:  fixedpoint.NewFromFloat(0.0),
			quoteBalance: fixedpoint.NewFromFloat(10000),
			want: &InventorySkewBidAskRatios{
				BidRatio: fixedpoint.NewFromFloat(2.0),
				AskRatio: fixedpoint.NewFromFloat(0.0),
			},
		},
		{
			quantity:     fixedpoint.NewFromFloat(1.0),
			price:        fixedpoint.NewFromFloat(1000),
			baseBalance:  fixedpoint.NewFromFloat(2.0),
			quoteBalance: fixedpoint.NewFromFloat(0.0),
			want: &InventorySkewBidAskRatios{
				BidRatio: fixedpoint.NewFromFloat(0.0),
				AskRatio: fixedpoint.NewFromFloat(2.0),
			},
		},
	}

	for _, c := range cases {
		s := &InventorySkew{
			InventoryRangeMultiplier: fixedpoint.NewFromFloat(0.1),
			TargetBaseRatio:          fixedpoint.NewFromFloat(0.5),
		}
		got := s.CalculateBidAskRatios(c.quantity, c.price, c.baseBalance, c.quoteBalance)
		assert.Equal(t, c.want.BidRatio.Float64(), got.BidRatio.Float64())
		assert.Equal(t, c.want.AskRatio.Float64(), got.AskRatio.Float64())
	}
}
