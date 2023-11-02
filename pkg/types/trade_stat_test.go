package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestCAGR(t *testing.T) {
	giveInitial := 1000.0
	giveFinal := 2500.0
	giveDays := 190
	want := 4.81
	act := CAGR(giveInitial, giveFinal, giveDays)
	assert.InDelta(t, want, act, 0.01)
}

func TestKellyCriterion(t *testing.T) {
	var (
		giveProfitFactor = fixedpoint.NewFromFloat(1.6)
		giveWinP         = fixedpoint.NewFromFloat(0.7)
		want             = 0.51
		act              = KellyCriterion(giveProfitFactor, giveWinP)
	)
	assert.InDelta(t, want, act.Float64(), 0.01)
}

func TestAnnualHistoricVolatility(t *testing.T) {
	var (
		give = floats.Slice{0.1, 0.2, -0.15, 0.1, 0.8, -0.3, 0.2}
		want = 5.51
		act  = AnnualHistoricVolatility(give)
	)
	assert.InDelta(t, want, act, 0.01)
}

func TestOptimalF(t *testing.T) {
	roundturns := floats.Slice{10, 20, 50, -10, 40, -40}
	f := OptimalF(roundturns)
	assert.EqualValues(t, 0.45, f)
}

func TestDrawdown(t *testing.T) {
	roundturns := floats.Slice{100, 50, 100}
	expected := []float64{.0, .5, .0}
	drawdown := Drawdown(roundturns)
	assert.EqualValues(t, 0.5, drawdown.Max())
	assert.EqualValues(t, 0.16666666666666666, drawdown.Average())
	assert.EqualValues(t, 0.08333333333333333, drawdown.AverageSquared())
	for i, v := range expected {
		assert.EqualValues(t, v, drawdown[i])
	}
}
