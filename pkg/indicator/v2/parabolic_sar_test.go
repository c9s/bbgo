package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestParabolicSAR(t *testing.T) {
	ts := []types.KLine{
		{Open: n(3836.86), High: n(3836.86), Low: n(3643.25), Close: n(3790.55)},
		{Open: n(3766.57), High: n(3766.57), Low: n(3542.73), Close: n(3546.20)},
		{Open: n(3576.17), High: n(3576.17), Low: n(3371.75), Close: n(3507.31)},
		{Open: n(3513.55), High: n(3513.55), Low: n(3334.02), Close: n(3340.81)},
		{Open: n(3529.75), High: n(3529.75), Low: n(3314.75), Close: n(3529.60)},
		{Open: n(3756.17), High: n(3756.17), Low: n(3558.21), Close: n(3717.41)},
		{Open: n(3717.17), High: n(3717.17), Low: n(3517.79), Close: n(3544.35)},
		{Open: n(3572.62), High: n(3572.62), Low: n(3447.90), Close: n(3478.14)},
		{Open: n(3612.43), High: n(3612.43), Low: n(3494.39), Close: n(3612.08)},
	}

	expectedValues := []float64{
		3836.86,
		3836.86,
		3836.86,
		3808.95,
		3770.96,
		3314.75,
		3314.75,
		3323.58,
		3332.23,
	}

	expectedTrend := []Trend{
		Falling,
		Falling,
		Falling,
		Falling,
		Falling,
		Rising,
		Rising,
		Rising,
		Rising,
	}

	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := ParabolicSar(kLines)
	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	for i, v := range expectedValues {
		assert.InDelta(t, v, ind.Slice[i], 0.01, "Expected PSAR.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}

	for i, v := range expectedTrend {
		assert.Equal(t, v, ind.Trend[i], "Expected PSAR.Trend[%d] to be %v, but got %v", i, v, ind.Trend[i])
	}
}
