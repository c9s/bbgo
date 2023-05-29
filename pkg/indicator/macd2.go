package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

/*
NEW INDICATOR DESIGN:

klines := kLines(marketDataStream)
closePrices := closePrices(klines)
macd := MACD(klines, {Fast: 12, Slow: 10})

equals to:

klines := KLines(marketDataStream)
closePrices := ClosePrice(klines)
fastEMA := EMA(closePrices, 7)
slowEMA := EMA(closePrices, 25)
macd := Subtract(fastEMA, slowEMA)
signal := EMA(macd, 16)
histogram := Subtract(macd, signal)
*/

type Float64Source interface {
	types.Series
	OnUpdate(f func(v float64))
}

type Float64Subscription interface {
	types.Series
	AddSubscriber(f func(v float64))
}

//go:generate callbackgen -type EWMAStream
type EWMAStream struct {
	Float64Updater
	types.SeriesBase

	slice floats.Slice

	window     int
	multiplier float64
}

func EWMA2(source Float64Source, window int) *EWMAStream {
	s := &EWMAStream{
		window:     window,
		multiplier: 2.0 / float64(1+window),
	}

	s.SeriesBase.Series = s.slice

	if sub, ok := source.(Float64Subscription); ok {
		sub.AddSubscriber(s.calculateAndPush)
	} else {
		source.OnUpdate(s.calculateAndPush)
	}

	return s
}

func (s *EWMAStream) calculateAndPush(v float64) {
	v2 := s.calculate(v)
	s.slice.Push(v2)
	s.EmitUpdate(v2)
}

func (s *EWMAStream) calculate(v float64) float64 {
	last := s.slice.Last()
	m := s.multiplier
	return (1.0-m)*last + m*v
}

type SubtractStream struct {
	Float64Updater
	types.SeriesBase

	a, b, c floats.Slice
	i       int
}

func Subtract(a, b Float64Source) *SubtractStream {
	s := &SubtractStream{}
	s.SeriesBase.Series = s.c

	a.OnUpdate(func(v float64) {
		s.a.Push(v)
		s.calculate()
	})
	b.OnUpdate(func(v float64) {
		s.b.Push(v)
		s.calculate()
	})
	return s
}

func (s *SubtractStream) calculate() {
	if s.a.Length() != s.b.Length() {
		return
	}

	if s.a.Length() > s.c.Length() {
		var numNewElems = s.a.Length() - s.c.Length()
		var tailA = s.a.Tail(numNewElems)
		var tailB = s.b.Tail(numNewElems)
		var tailC = tailA.Sub(tailB)
		for _, f := range tailC {
			s.c.Push(f)
			s.EmitUpdate(f)
		}
	}
}
