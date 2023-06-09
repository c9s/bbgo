package indicator

type BollStream struct {
	// the band series
	*Float64Series

	UpBand, DownBand *Float64Series

	window int
	k      float64

	SMA    *SMAStream
	StdDev *StdDevStream
}

// BOOL2 is bollinger indicator
// the data flow:
//
// priceSource ->
//
//	-> calculate SMA
//	-> calculate stdDev -> calculate bandWidth -> get latest SMA -> upBand, downBand
func BOLL2(source Float64Source, window int, k float64) *BollStream {
	// bind these indicators before our main calculator
	sma := SMA2(source, window)
	stdDev := StdDev2(source, window)

	s := &BollStream{
		Float64Series: NewFloat64Series(),
		UpBand:        NewFloat64Series(),
		DownBand:      NewFloat64Series(),
		window:        window,
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

func (s *BollStream) Calculate(v float64) float64 {
	stdDev := s.StdDev.Last(0)
	band := stdDev * s.k
	return band
}
