package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestPremiumSignal(t *testing.T) {
	sig := PremiumSignalStream{
		Float64Series: types.NewFloat64Series(),
		premiumMargin: 0.01,
	}
	var p_prime float64
	sig.OnUpdate(func(v float64) {
		p_prime = v
	})
	// should be neutral when there is no prices yet
	p := sig.Last(0)
	assert.Equal(t, 0.0, p)

	sig.price1 = 1.05
	sig.price2 = 1.0
	sig.calculatePremium()
	p = sig.Last(0)
	assert.Equal(t, 1.05, p)
	assert.Equal(t, p, p_prime)

	sig.price1 = 1.0
	sig.price2 = 1.5
	sig.calculatePremium()
	p = sig.Last(0)
	assert.Equal(t, 1.0/1.5, p)
	assert.Equal(t, p, p_prime)

	sig.price2 = 1.005
	sig.calculatePremium()
	p = sig.Last(0)
	assert.Equal(t, 0.0, p)
	assert.Equal(t, p, p_prime)
}
