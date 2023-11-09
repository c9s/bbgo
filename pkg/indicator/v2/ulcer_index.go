package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

type UlcerIndexStream struct {
	*types.Float64Series
	sma *SMAStream
}

// The Ulcer Index (UI) measures downside risk. The index increases in value
// as the price moves farther away from a recent high and falls as the price
// rises to new highs.
//
// High Closings = Max(period, Closings)
// Percentage Drawdown = 100 * ((Closings - High Closings) / High Closings)
// Squared Average = Sma(period, Percent Drawdown * Percent Drawdown)
// Ulcer Index = Sqrt(Squared Average)

// https://www.investopedia.com/terms/a/UlcerIndex.asp
func UlcerIndex(source types.Float64Source, window int) *UlcerIndexStream {
	s := &UlcerIndexStream{
		Float64Series: types.NewFloat64Series(),
		sma:           SMA(SquaredAverage(source, window), window),
	}

	s.Bind(source, s)

	return s
}

// The default ulcer index with the default period of 14.
func UlcerIndexDefault(source types.Float64Source) *UlcerIndexStream {
	return UlcerIndex(source, 14)
}

func (s *UlcerIndexStream) Calculate(_ float64) float64 {
	return math.Sqrt(s.sma.Last(0))
}

func (s *UlcerIndexStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfMA)
}
