package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestPremiumSignal(t *testing.T) {
	base := "NOTREAL"
	quote1 := "USDT"
	quote2 := "USDC"

	symbol1 := base + quote1
	symbol2 := base + quote2

	klines := []types.KLine{
		{
			Symbol:   symbol1,
			Interval: types.Interval1m,
			Close:    fixedpoint.NewFromFloat(1.1),
		},
		{
			Symbol:   symbol2,
			Interval: types.Interval1m,
			Close:    fixedpoint.NewFromFloat(1.0),
		},
	}

	sig := PremiumSignal(
		nil,
		base,
		quote1,
		quote2,
		types.Interval1m,
		0.01,
	)
	// should be neutral when there is no klines yet
	p := sig.Last(0)
	assert.Equal(t, Neutral, PremiumType(p))

	sig.BackFill(klines)
	p = sig.Last(0)
	assert.Equal(t, Premium, PremiumType(p))

	sig.BackFill([]types.KLine{
		{
			Symbol:   symbol2,
			Interval: types.Interval1m,
			Close:    fixedpoint.NewFromFloat(1.5),
		},
	})
	p = sig.Last(0)
	assert.Equal(t, Discount, PremiumType(p))

	sig.BackFill([]types.KLine{
		{
			Symbol:   symbol2,
			Interval: types.Interval1m,
			Close:    fixedpoint.NewFromFloat(1.09),
		},
	})
	p = sig.Last(0)
	assert.Equal(t, Neutral, PremiumType(p))

}
