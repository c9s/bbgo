package bbgo

import (
	"encoding/json"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

type SourceFunc func(*types.KLine) fixedpoint.Value

type selectorInternal struct {
	Source       string
	sourceGetter SourceFunc
}

func (s *selectorInternal) UnmarshalJSON(d []byte) error {
	if err := json.Unmarshal(d, &s.Source); err != nil {
		return err
	}
	s.init()
	return nil
}

func (s selectorInternal) MarshalJSON() ([]byte, error) {
	if s.Source == "" {
		s.Source = "close"
		s.init()
	}
	return []byte("\"" + s.Source + "\""), nil
}

type SourceSelector struct {
	Source selectorInternal `json:"source,omitempty"`
}

func (s *selectorInternal) init() {
	switch strings.ToLower(s.Source) {
	case "close":
		s.sourceGetter = func(kline *types.KLine) fixedpoint.Value { return kline.Close }
	case "high":
		s.sourceGetter = func(kline *types.KLine) fixedpoint.Value { return kline.High }
	case "low":
		s.sourceGetter = func(kline *types.KLine) fixedpoint.Value { return kline.Low }
	case "hl2":
		s.sourceGetter = func(kline *types.KLine) fixedpoint.Value { return kline.High.Add(kline.Low).Div(fixedpoint.Two) }
	case "hlc3":
		s.sourceGetter = func(kline *types.KLine) fixedpoint.Value {
			return kline.High.Add(kline.Low).Add(kline.Close).Div(fixedpoint.Three)
		}
	case "ohlc4":
		s.sourceGetter = func(kline *types.KLine) fixedpoint.Value {
			return kline.High.Add(kline.Low).Add(kline.Close).Add(kline.Open).Div(fixedpoint.Four)
		}
	case "open":
		s.sourceGetter = func(kline *types.KLine) fixedpoint.Value { return kline.Open }
	case "oc2":
		s.sourceGetter = func(kline *types.KLine) fixedpoint.Value { return kline.Open.Add(kline.Close).Div(fixedpoint.Two) }
	default:
		log.Infof("source not set: %s, use hl2 by default", s.Source)
		s.sourceGetter = func(kline *types.KLine) fixedpoint.Value { return kline.High.Add(kline.Low).Div(fixedpoint.Two) }
	}
}

func (s *selectorInternal) String() string {
	if s.Source == "" {
		s.Source = "close"
		s.init()
	}
	return s.Source
}

// lazy init if empty struct is passed in
func (s *SourceSelector) GetSource(kline *types.KLine) fixedpoint.Value {
	if s.Source.Source == "" {
		s.Source.Source = "close"
		s.Source.init()
	}
	return s.Source.sourceGetter(kline)
}
