package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type BOLLStream struct {
	// the band series
	*types.Float64Series

	UpBand, DownBand *types.Float64Series

	k float64

	SMA    *SMAStream
	StdDev *StdDevStream
}

// BOLL2 is bollinger indicator
// the data flow:
//
// priceSource ->
//
//	-> calculate SMA
//	-> calculate stdDev -> calculate bandWidth -> get latest SMA -> upBand, downBand
func BOLL(source types.Float64Source, window int, k float64) *BOLLStream {
	// bind these indicators before our main calculator
	sma := SMA(source, window)
	stdDev := StdDev(source, window)

	s := &BOLLStream{
		Float64Series: types.NewFloat64Series(),
		UpBand:        types.NewFloat64Series(),
		DownBand:      types.NewFloat64Series(),
		k:             k,
		SMA:           sma,
		StdDev:        stdDev,
	}
	s.Bind(source, s)

	// on band update
	s.Float64Series.OnUpdate(func(band float64) {
		mid := s.SMA.Last(0)
		s.UpBand.PushAndEmit(mid + band)
		s.DownBand.PushAndEmit(mid - band)
	})
	return s
}

func (s *BOLLStream) Calculate(v float64) float64 {
	stdDev := s.StdDev.Last(0)
	band := stdDev * s.k
	return band
}

func (s *BOLLStream) Truncate() {
	s.UpBand.Slice = s.UpBand.Slice.Truncate(5000)
	s.DownBand.Slice = s.DownBand.Slice.Truncate(5000)
}
