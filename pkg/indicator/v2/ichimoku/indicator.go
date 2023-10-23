package ichimoku

import (
	"fmt"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type IchimokuStream struct {
	*types.KLineSeries
	line_helper    lineHelper
	Ichimokus      []*IchimokuStatus
	ConversionLine *types.Float64Series
	BaseLine       *types.Float64Series
	LeadingSpanA   *types.Float64Series
	LeadingSpanB   *types.Float64Series
	laggingSpan    *types.Float64Series
	window         int
}

func Ichimoku(source v2.KLineSubscription, window int) *IchimokuStream {
	s := &IchimokuStream{
		KLineSeries: v2.KlineSeries(),
		window:      window,
	}
	s.Bind(source, s)
	return s
}

func (s *IchimokuStream) Calculate(v float64) float64 {
	bars_len := s.Length()

	if bars_len < numberOfRead || bars_len < 52 {
		return ErrNotEnoughData
	}

	fmt.Printf("Calc ichi from Last %v days on %d total candles\r\n", numberOfRead, len(series))

	// descending
	for day := 0; day < numberOfRead; day++ {

		from := bars_len - 52 - day
		to := bars_len - day

		ic, err := BuildIchimokuStatus(s.loadbars(from, to))
		if err != nil {
			return err
		}
		s.Put(ic)

	}

	for day_index := 0; day_index < s.NumberOfIchimokus(); day_index++ {
		item := s.Ichimokus[day_index]
		sen_a, sen_b := s.Calc_Cloud_InPast(day_index)
		item.Set_SenCo_A_Past(sen_a)
		item.Set_SenCo_B_Past(sen_b)
	}

	return sma
}

func (s *IchimokuStream) loadbars(from int, to int) []types.KLine {
	if s.Length() == 0 {
		return nil
	}
	if from < 0 {
		from = 0
	}
	if to > s.Length() {
		to = s.Length()
	}

	return s.Last(0) // (from:to)

}
