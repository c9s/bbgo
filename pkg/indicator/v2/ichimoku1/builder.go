package ichimoku

import (
	"github.com/algo-boyz/alphakit/market"

	"github.com/algo-boyz/alphakit/ta/dec"
)

func BuildIchimokuStatus(bars []*market.Kline) (*IchimokuStatus, error) {
	if len(bars) == 0 {
		return nil, ErrDataNotFill
	}

	if len(bars) < 52 {
		return nil, ErrNotEnoughData
	}

	tenkenLine := calcLine(Line_Tenkan_sen, bars)
	kijonLine := calcLine(Line_kijon_sen, bars)

	span_a := calculate_span_a(tenkenLine, kijonLine)
	span_b := calcLine(Line_spanPeriod, bars)
	chiko_index := (len(bars) - int(Line_chikoPeriod)) - 1
	cheko_span := bars[chiko_index]

	var latestPrice *market.Kline
	latestPriceIndex := (len(bars) - 1)
	if (len(bars) - 1) >= latestPriceIndex {
		latestPrice = bars[latestPriceIndex]
	}

	if !tenkenLine.isNil && !kijonLine.isNil && !span_a.isNil && !span_b.isNil {
		ichi := NewIchimokuStatus(tenkenLine, kijonLine, span_a, span_b, cheko_span, latestPrice)
		return ichi, nil
	}

	return nil, ErrBuildFailed
}

func calcLine(line_type ELine, bars []*market.Kline) ValueLine {
	high := NewValueLineNil()
	low := NewValueLineNil()
	l := len(bars)
	from := l - 1 - int(line_type)
	if from == -1 {
		from = 0
	}
	bars_tmp := bars[from : l-1]
	if len(bars) < int(line_type) {
		return NewValueLineNil()
	}
	for _, v := range bars_tmp {

		if high.isNil {
			high.SetValue(v.H)
		}
		if low.isNil {
			low.SetValue(v.L)
		}

		if v.H.GreaterThan(high.valLine) {
			high.SetValue(v.H)
		}

		if v.L.LessThan(low.valLine) {
			low.SetValue(v.L)
		}

	}
	line := low.valLine.Add(high.valLine).Quo(dec.New(2))
	return NewValue(line)
}

func calculate_span_a(tenken ValueLine, kijon ValueLine) ValueLine {
	if !tenken.isNil && !kijon.isNil {
		v := tenken.valLine.Add(kijon.valLine).Quo(dec.New(2))
		return NewValue(v)
	}

	return NewValueLineNil()
}
