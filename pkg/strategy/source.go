package strategy

import (
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

type SourceFunc func(*types.KLine) fixedpoint.Value

var Four fixedpoint.Value = fixedpoint.NewFromInt(4)
var Three fixedpoint.Value = fixedpoint.NewFromInt(3)
var Two fixedpoint.Value = fixedpoint.NewFromInt(2)

type SourceSelector struct {
	Source    string `json:"source,omitempty"`
	getSource SourceFunc
}

func (s *SourceSelector) Init() {
	switch strings.ToLower(s.Source) {
	case "close":
		s.getSource = func(kline *types.KLine) fixedpoint.Value { return kline.Close }
	case "high":
		s.getSource = func(kline *types.KLine) fixedpoint.Value { return kline.High }
	case "low":
		s.getSource = func(kline *types.KLine) fixedpoint.Value { return kline.Low }
	case "hl2":
		s.getSource = func(kline *types.KLine) fixedpoint.Value { return kline.High.Add(kline.Low).Div(Two) }
	case "hlc3":
		s.getSource = func(kline *types.KLine) fixedpoint.Value {
			return kline.High.Add(kline.Low).Add(kline.Close).Div(Three)
		}
	case "ohlc4":
		s.getSource = func(kline *types.KLine) fixedpoint.Value {
			return kline.High.Add(kline.Low).Add(kline.Close).Add(kline.Open).Div(Four)
		}
	case "open":
		s.getSource = func(kline *types.KLine) fixedpoint.Value { return kline.Open }
	default:
		log.Infof("source not set: %s, use hl2 by default", s.Source)
		s.getSource = func(kline *types.KLine) fixedpoint.Value { return kline.High.Add(kline.Low).Div(Two) }
	}
}

func (s *SourceSelector) GetSource(kline *types.KLine) fixedpoint.Value {
	return s.getSource(kline)
}
