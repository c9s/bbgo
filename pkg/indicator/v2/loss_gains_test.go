package indicatorv2

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestGainIndicator(t *testing.T) {

	closing := []float64{44.3389, 44.0902, 44.1497, 43.6124, 44.3278, 44.8264, 45.0955, 45.4245, 45.8433, 46.0826, 45.8931, 46.0328, 45.6140, 46.2820, 46.2820, 46.0028, 46.0328, 46.4116, 46.2222, 45.6439, 46.2122, 46.2521, 45.7137, 46.4515, 45.7835, 45.3548, 44.0288, 44.1783, 44.2181, 44.5672, 43.4205, 42.6628, 43.1314}
	expected := []float64{.24, .22, .21, .22, 0.20, .19, .22, .20, .19, .23, .21, .20, .18, .18, .17, .18, .17, .16, .18}

	ts := buildKLinesFromC(closing)
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := GainLoss(kLines, GainCoefficient)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	spew.Dump(ind)
	for i, v := range expected {
		assert.InDelta(t, v, ind.Slice[i], 0.01, "Expected Gain.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}
}

func TestLossIndicator(t *testing.T) {
	ts := []types.KLine{
		{Close: n(1)},
		{Close: n(2)},
		{Close: n(3)},
		{Close: n(3)},
		{Close: n(2)},
		{Close: n(0)},
	}
	expected := []float64{0, 0, 0, 0, 1, 2}

	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := GainLoss(kLines, LossCoefficient)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	spew.Dump(ind)
	for i, v := range expected {
		assert.InDelta(t, v, ind.Slice[i], 0.01, "Expected Loss.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}
}

// func TestCumulativeGainsIndicator(t *testing.T) {
// 	t.Run("Basic", func(t *testing.T) {
// 		ts := mockTimeSeriesFl(1, 2, 3, 5, 8, 13)

// 		cumGains := NewCumulativeGainsIndicator(NewCloseIndicator(ts), 6)

// 		decimalEquals(t, 0, cumGains.Calculate(0))
// 		decimalEquals(t, 1, cumGains.Calculate(1))
// 		decimalEquals(t, 2, cumGains.Calculate(2))
// 		decimalEquals(t, 4, cumGains.Calculate(3))
// 		decimalEquals(t, 7, cumGains.Calculate(4))
// 		decimalEquals(t, 12, cumGains.Calculate(5))
// 	})

// 	t.Run("Oscillating scale", func(t *testing.T) {
// 		ts := mockTimeSeriesFl(0, 5, 2, 10, 12, 11)

// 		cumGains := NewCumulativeGainsIndicator(NewCloseIndicator(ts), 6)

// 		decimalEquals(t, 0, cumGains.Calculate(0))
// 		decimalEquals(t, 5, cumGains.Calculate(1))
// 		decimalEquals(t, 5, cumGains.Calculate(2))
// 		decimalEquals(t, 13, cumGains.Calculate(3))
// 		decimalEquals(t, 15, cumGains.Calculate(4))
// 		decimalEquals(t, 15, cumGains.Calculate(5))
// 	})

// 	t.Run("Rolling timeframe", func(t *testing.T) {
// 		ts := mockTimeSeriesFl(0, 5, 2, 10, 12, 11)

// 		cumGains := NewCumulativeGainsIndicator(NewCloseIndicator(ts), 3)

// 		decimalEquals(t, 0, cumGains.Calculate(0))
// 		decimalEquals(t, 5, cumGains.Calculate(1))
// 		decimalEquals(t, 5, cumGains.Calculate(2))
// 		decimalEquals(t, 13, cumGains.Calculate(3))
// 		decimalEquals(t, 10, cumGains.Calculate(4))
// 		decimalEquals(t, 10, cumGains.Calculate(5))
// 	})
// }

// func TestCumulativeLossesIndicator(t *testing.T) {
// 	t.Run("Basic", func(t *testing.T) {
// 		ts := mockTimeSeriesFl(13, 8, 5, 3, 2, 1)

// 		cumLosses := NewCumulativeLossesIndicator(NewCloseIndicator(ts), 6)

// 		decimalEquals(t, 0, cumLosses.Calculate(0))
// 		decimalEquals(t, 5, cumLosses.Calculate(1))
// 		decimalEquals(t, 8, cumLosses.Calculate(2))
// 		decimalEquals(t, 10, cumLosses.Calculate(3))
// 		decimalEquals(t, 11, cumLosses.Calculate(4))
// 		decimalEquals(t, 12, cumLosses.Calculate(5))
// 	})

// 	t.Run("Oscillating indicator", func(t *testing.T) {
// 		ts := mockTimeSeriesFl(13, 16, 10, 8, 9, 8)

// 		cumLosses := NewCumulativeLossesIndicator(NewCloseIndicator(ts), 6)

// 		decimalEquals(t, 0, cumLosses.Calculate(0))
// 		decimalEquals(t, 0, cumLosses.Calculate(1))
// 		decimalEquals(t, 6, cumLosses.Calculate(2))
// 		decimalEquals(t, 8, cumLosses.Calculate(3))
// 		decimalEquals(t, 8, cumLosses.Calculate(4))
// 		decimalEquals(t, 9, cumLosses.Calculate(5))
// 	})

// 	t.Run("Rolling timeframe", func(t *testing.T) {
// 		ts := mockTimeSeriesFl(13, 16, 10, 8, 9, 8)

// 		cumLosses := NewCumulativeLossesIndicator(NewCloseIndicator(ts), 3)

// 		decimalEquals(t, 0, cumLosses.Calculate(0))
// 		decimalEquals(t, 0, cumLosses.Calculate(1))
// 		decimalEquals(t, 6, cumLosses.Calculate(2))
// 		decimalEquals(t, 8, cumLosses.Calculate(3))
// 		decimalEquals(t, 8, cumLosses.Calculate(4))
// 		decimalEquals(t, 3, cumLosses.Calculate(5))
// 	})
// }

// func TestPercentGainIndicator(t *testing.T) {
// 	t.Run("Up", func(t *testing.T) {
// 		ts := mockTimeSeriesFl(1, 1.5, 2.25, 2.25)

// 		pgi := NewPercentChangeIndicator(NewCloseIndicator(ts))

// 		decimalEquals(t, 0, pgi.Calculate(0))
// 		decimalEquals(t, .5, pgi.Calculate(1))
// 		decimalEquals(t, .5, pgi.Calculate(2))
// 		decimalEquals(t, 0, pgi.Calculate(3))
// 	})

// 	t.Run("Down", func(t *testing.T) {
// 		ts := mockTimeSeriesFl(10, 5, 2.5, 2.5)

// 		pgi := NewPercentChangeIndicator(NewCloseIndicator(ts))

// 		decimalEquals(t, 0, pgi.Calculate(0))
// 		decimalEquals(t, -.5, pgi.Calculate(1))
// 		decimalEquals(t, -.5, pgi.Calculate(2))
// 		decimalEquals(t, 0, pgi.Calculate(3))
// 	})
// }
