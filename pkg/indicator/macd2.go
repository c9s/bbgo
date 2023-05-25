package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type KLineStream
type KLineStream struct {
	updateCallbacks []func(k types.KLine)
}

func KLines(source types.Stream) *KLineStream {
	stream := &KLineStream{}
	source.OnKLineClosed(stream.EmitUpdate)
	return stream
}

type KLineSource interface {
	OnUpdate(f func(k types.KLine))
}

type PriceStream struct {
	types.SeriesBase
	Float64Updater

	slice  floats.Slice
	mapper KLineValueMapper
}

func Price(source KLineSource, mapper KLineValueMapper) *PriceStream {
	s := &PriceStream{
		mapper: mapper,
	}
	s.SeriesBase.Series = s.slice
	source.OnUpdate(func(k types.KLine) {
		v := s.mapper(k)
		s.slice.Push(v)
		s.EmitUpdate(v)
	})
	return s
}

func ClosePrices(source KLineSource) *PriceStream {
	return Price(source, KLineClosePriceMapper)
}

func OpenPrices(source KLineSource) *PriceStream {
	return Price(source, KLineOpenPriceMapper)
}

type Float64Source interface {
	types.Series
	OnUpdate(f func(v float64))
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
	source.OnUpdate(func(v float64) {
		v2 := s.calculate(v)
		s.slice.Push(v2)
		s.EmitUpdate(v2)
	})
	return s
}

func (s *EWMAStream) calculate(v float64) float64 {
	last := s.slice.Last()
	m := s.multiplier
	return (1.0-m)*last + m*v
}

//go:generate callbackgen -type Float64Updater
type Float64Updater struct {
	updateCallbacks []func(v float64)
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
