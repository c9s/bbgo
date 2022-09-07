package elliottwave

import "github.com/c9s/bbgo/pkg/indicator"

type ElliottWave struct {
	maSlow  *indicator.SMA
	maQuick *indicator.SMA
}

func (s *ElliottWave) Index(i int) float64 {
	return s.maQuick.Index(i)/s.maSlow.Index(i) - 1.0
}

func (s *ElliottWave) Last() float64 {
	return s.maQuick.Last()/s.maSlow.Last() - 1.0
}

func (s *ElliottWave) Length() int {
	return s.maSlow.Length()
}

func (s *ElliottWave) Update(v float64) {
	s.maSlow.Update(v)
	s.maQuick.Update(v)
}
