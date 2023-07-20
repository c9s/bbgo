package rsicross

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "rsicross"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment
	Market      types.Market

	Symbol     string           `json:"symbol"`
	Interval   types.Interval   `json:"interval"`
	SlowWindow int              `json:"slowWindow"`
	FastWindow int              `json:"fastWindow"`
	OpenBelow  fixedpoint.Value `json:"openBelow"`
	CloseAbove fixedpoint.Value `json:"closeAbove"`

	bbgo.OpenPositionOptions
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s:%s:%d-%d", ID, s.Symbol, s.Interval, s.FastWindow, s.SlowWindow)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy = &common.Strategy{}
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	fastRsi := session.Indicators(s.Symbol).RSI(types.IntervalWindow{Interval: s.Interval, Window: s.FastWindow})
	slowRsi := session.Indicators(s.Symbol).RSI(types.IntervalWindow{Interval: s.Interval, Window: s.SlowWindow})
	rsiCross := indicatorv2.Cross(fastRsi, slowRsi)
	rsiCross.OnUpdate(func(v float64) {
		switch indicatorv2.CrossType(v) {
		case indicatorv2.CrossOver:
			if s.OpenBelow.Sign() > 0 && fastRsi.Last(0) > s.OpenBelow.Float64() {
				return
			}

			opts := s.OpenPositionOptions
			opts.Long = true

			if price, ok := session.LastPrice(s.Symbol); ok {
				opts.Price = price
			}

			// opts.Price = closePrice
			opts.Tags = []string{"rsiCrossOver"}
			if _, err := s.OrderExecutor.OpenPosition(ctx, opts); err != nil {
				logErr(err, "unable to open position")
			}

		case indicatorv2.CrossUnder:
			if s.CloseAbove.Sign() > 0 && fastRsi.Last(0) < s.CloseAbove.Float64() {
				return
			}

			if err := s.OrderExecutor.ClosePosition(ctx, fixedpoint.One); err != nil {
				logErr(err, "failed to close position")
			}

		}
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
	})

	return nil
}

func logErr(err error, msgAndArgs ...interface{}) bool {
	if err == nil {
		return false
	}

	if len(msgAndArgs) == 0 {
		log.WithError(err).Error(err.Error())
	} else if len(msgAndArgs) == 1 {
		msg := msgAndArgs[0].(string)
		log.WithError(err).Error(msg)
	} else if len(msgAndArgs) > 1 {
		msg := msgAndArgs[0].(string)
		log.WithError(err).Errorf(msg, msgAndArgs[1:]...)
	}

	return true
}
