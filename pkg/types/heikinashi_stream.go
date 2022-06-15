package types

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var Four fixedpoint.Value = fixedpoint.NewFromInt(4)

type HeikinAshiStream struct {
	StandardStreamEmitter
	lastAshi   map[string]map[Interval]*KLine
	LastOrigin map[string]map[Interval]*KLine
}

func (s *HeikinAshiStream) EmitKLineClosed(kline KLine) {
	ashi := kline
	if s.lastAshi == nil {
		s.lastAshi = make(map[string]map[Interval]*KLine)
		s.LastOrigin = make(map[string]map[Interval]*KLine)
	}
	if s.lastAshi[kline.Symbol] == nil {
		s.lastAshi[kline.Symbol] = make(map[Interval]*KLine)
		s.LastOrigin[kline.Symbol] = make(map[Interval]*KLine)
	}
	lastAshi := s.lastAshi[kline.Symbol][kline.Interval]
	if lastAshi == nil {
		ashi.Close = kline.Close.Add(kline.High).
			Add(kline.Low).
			Add(kline.Open).
			Div(Four)
		// High and Low are the same
		s.lastAshi[kline.Symbol][kline.Interval] = &ashi
		s.LastOrigin[kline.Symbol][kline.Interval] = &kline
	} else {
		ashi.Close = kline.Close.Add(kline.High).
			Add(kline.Low).
			Add(kline.Open).
			Div(Four)
		ashi.Open = lastAshi.Open.Add(lastAshi.Close).Div(Two)
		// High and Low are the same
		s.lastAshi[kline.Symbol][kline.Interval] = &ashi
		s.LastOrigin[kline.Symbol][kline.Interval] = &kline
	}
	s.StandardStreamEmitter.EmitKLineClosed(ashi)
}

// No writeback to lastAshi
func (s *HeikinAshiStream) EmitKLine(kline KLine) {
	ashi := kline
	if s.lastAshi == nil {
		s.lastAshi = make(map[string]map[Interval]*KLine)
	}
	if s.lastAshi[kline.Symbol] == nil {
		s.lastAshi[kline.Symbol] = make(map[Interval]*KLine)
	}
	lastAshi := s.lastAshi[kline.Symbol][kline.Interval]
	if lastAshi == nil {
		ashi.Close = kline.Close.Add(kline.High).
			Add(kline.Low).
			Add(kline.Open).
			Div(Four)
	} else {
		ashi.Close = kline.Close.Add(kline.High).
			Add(kline.Low).
			Add(kline.Open).
			Div(Four)
		ashi.Open = lastAshi.Open.Add(lastAshi.Close).Div(Two)
	}
	s.StandardStreamEmitter.EmitKLine(ashi)
}

var _ StandardStreamEmitter = &HeikinAshiStream{}
