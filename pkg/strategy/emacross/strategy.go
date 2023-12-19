package emacross

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

const ID = "emacross"

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

	lastKLine types.KLine

	bbgo.OpenPositionOptions
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s:%s:%d-%d", ID, s.Symbol, s.Interval, s.FastWindow, s.SlowWindow)
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval5m, func(k types.KLine) {
		s.lastKLine = k
	}))

	fastEMA := session.Indicators(s.Symbol).EWMA(types.IntervalWindow{Interval: s.Interval, Window: s.FastWindow})
	slowEMA := session.Indicators(s.Symbol).EWMA(types.IntervalWindow{Interval: s.Interval, Window: s.SlowWindow})

	cross := indicatorv2.Cross(fastEMA, slowEMA)
	cross.OnUpdate(func(v float64) {
		switch indicatorv2.CrossType(v) {

		case indicatorv2.CrossOver:
			if err := s.Strategy.OrderExecutor.GracefulCancel(ctx); err != nil {
				log.WithError(err).Errorf("unable to cancel order")
			}

			opts := s.OpenPositionOptions
			opts.Long = true
			if price, ok := session.LastPrice(s.Symbol); ok {
				opts.Price = price
			}

			opts.Tags = []string{"emaCrossOver"}

			_, err := s.Strategy.OrderExecutor.OpenPosition(ctx, opts)
			logErr(err, "unable to open position")
		case indicatorv2.CrossUnder:
			err := s.Strategy.OrderExecutor.ClosePosition(ctx, fixedpoint.One)
			logErr(err, "unable to submit close position order")
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
