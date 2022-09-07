package elliottwave

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type HeikinAshi struct {
	Values []types.KLine
	size   int
}

func NewHeikinAshi(size int) *HeikinAshi {
	return &HeikinAshi{
		Values: make([]types.KLine, size),
		size:   size,
	}
}

func (inc *HeikinAshi) Last() *types.KLine {
	if len(inc.Values) == 0 {
		return &types.KLine{}
	}
	return &inc.Values[len(inc.Values)-1]
}

func (inc *HeikinAshi) Update(kline types.KLine) {
	open := kline.Open
	cloze := kline.Close
	high := kline.High
	low := kline.Low
	lastOpen := inc.Last().Open
	lastClose := inc.Last().Close

	newClose := open.Add(high).Add(low).Add(cloze).Div(Four)
	newOpen := lastOpen.Add(lastClose).Div(Two)

	kline.Close = newClose
	kline.Open = newOpen
	kline.High = fixedpoint.Max(fixedpoint.Max(high, newOpen), newClose)
	kline.Low = fixedpoint.Max(fixedpoint.Min(low, newOpen), newClose)
	inc.Values = append(inc.Values, kline)
	if len(inc.Values) > inc.size {
		inc.Values = inc.Values[len(inc.Values)-inc.size:]
	}
}
