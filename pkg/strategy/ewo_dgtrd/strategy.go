package ewo_dgtrd

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "ewo_dgtrd"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Symbol       string         `json:"symbol"`
	Interval     types.Interval `json:"interval"`
	Threshold    float64        `json:"threshold"` // strength threshold
	UseEma       bool           `json:"useEma"`    // use exponential ma or simple ma
	SignalWindow int            `json:"sigWin"`    // signal window
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval.String()})
}

type EwoSignal interface {
	types.Series
	Update(value float64)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	indicatorSet, ok := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		log.Errorf("cannot get indicatorSet of %s", s.Symbol)
		return nil
	}
	var ma5, ma34, ewo types.Series
	if s.UseEma {
		ma5 = indicatorSet.EWMA(types.IntervalWindow{s.Interval, 5})
		ma34 = indicatorSet.EWMA(types.IntervalWindow{s.Interval, 34})
	} else {
		ma5 = indicatorSet.SMA(types.IntervalWindow{s.Interval, 5})
		ma34 = indicatorSet.SMA(types.IntervalWindow{s.Interval, 34})
	}
	ewo = types.Mul(types.Minus(types.Div(ma5, ma34), 1.0), 100.)
	var ewoSignal EwoSignal
	if s.UseEma {
		ewoSignal = &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, s.SignalWindow}}
	} else {
		ewoSignal = &indicator.SMA{IntervalWindow: types.IntervalWindow{s.Interval, s.SignalWindow}}
	}
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol {
			return
		}
		if ewoSignal.Length() == 0 {
			// lazy init
			ewoVals := types.ToReverseArray(ewo)
			for _, ewoValue := range ewoVals {
				ewoSignal.Update(ewoValue)
			}
		} else {
			ewoSignal.Update(ewo.Last())
		}

		lastPrice, ok := session.LastPrice(s.Symbol)
		if !ok {
			return
		}

		if types.CrossOver(ewo, ewoSignal).Last() {
			if ewo.Last() < -s.Threshold {
				// strong long
				log.Infof("strong long at %v", lastPrice)
			} else {
				log.Infof("long at %v", lastPrice)
				// Long
			}
		} else if types.CrossUnder(ewo, ewoSignal).Last() {
			if ewo.Last() > s.Threshold {
				// Strong short
				log.Infof("strong short at %v", lastPrice)
			} else {
				// short
				log.Infof("short at %v", lastPrice)
			}
		}
	})
	return nil
}
