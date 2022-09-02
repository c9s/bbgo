package elliottwave

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Strategy) DrawIndicators(store *bbgo.SerialMarketDataStore) *types.Canvas {
	klines, ok := store.KLinesOfInterval(types.Interval1m)
	if !ok {
		return nil
	}
	time := (*klines)[len(*klines)-1].StartTime
	canvas := types.NewCanvas(s.InstanceID(), s.Interval)
	Length := s.priceLines.Length()
	if Length > 300 {
		Length = 300
	}
	log.Infof("draw indicators with %d data", Length)
	mean := s.priceLines.Mean(Length)
	canvas.Plot("zero", types.NumberSeries(mean), time, Length)
	canvas.Plot("price", s.priceLines, time, Length)
	return canvas
}

func (s *Strategy) DrawPNL(profit types.Series) *types.Canvas {
	return nil
}

func (s *Strategy) DrawCumPNL(cumProfit types.Series) *types.Canvas {
	return nil
}

func (s *Strategy) Draw() {
}
