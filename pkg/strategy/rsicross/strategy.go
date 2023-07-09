package rsicross

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/risk/riskcontrol"
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

	Symbol     string         `json:"symbol"`
	Interval   types.Interval `json:"interval"`
	SlowWindow int            `json:"slowWindow"`
	FastWindow int            `json:"fastWindow"`

	bbgo.OpenPositionOptions

	// risk related parameters
	PositionHardLimit         fixedpoint.Value     `json:"positionHardLimit"`
	MaxPositionQuantity       fixedpoint.Value     `json:"maxPositionQuantity"`
	CircuitBreakLossThreshold fixedpoint.Value     `json:"circuitBreakLossThreshold"`
	CircuitBreakEMA           types.IntervalWindow `json:"circuitBreakEMA"`

	positionRiskControl     *riskcontrol.PositionRiskControl
	circuitBreakRiskControl *riskcontrol.CircuitBreakRiskControl
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

	if !s.PositionHardLimit.IsZero() && !s.MaxPositionQuantity.IsZero() {
		log.Infof("positionHardLimit and maxPositionQuantity are configured, setting up PositionRiskControl...")
		s.positionRiskControl = riskcontrol.NewPositionRiskControl(s.OrderExecutor, s.PositionHardLimit, s.MaxPositionQuantity)
	}

	if !s.CircuitBreakLossThreshold.IsZero() {
		log.Infof("circuitBreakLossThreshold is configured, setting up CircuitBreakRiskControl...")
		s.circuitBreakRiskControl = riskcontrol.NewCircuitBreakRiskControl(
			s.Position,
			session.Indicators(s.Symbol).EWMA(s.CircuitBreakEMA),
			s.CircuitBreakLossThreshold,
			s.ProfitStats)
	}

	fastRsi := session.Indicators(s.Symbol).RSI(types.IntervalWindow{Interval: s.Interval, Window: s.FastWindow})
	slowRsi := session.Indicators(s.Symbol).RSI(types.IntervalWindow{Interval: s.Interval, Window: s.SlowWindow})
	rsiCross := indicator.Cross(fastRsi, slowRsi)
	rsiCross.OnUpdate(func(v float64) {
		switch indicator.CrossType(v) {
		case indicator.CrossOver:
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

		case indicator.CrossUnder:
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

func (s *Strategy) preloadKLines(inc *indicator.KLineStream, session *bbgo.ExchangeSession, symbol string, interval types.Interval) {
	if store, ok := session.MarketDataStore(symbol); ok {
		if kLinesData, ok := store.KLinesOfInterval(interval); ok {
			for _, k := range *kLinesData {
				inc.EmitUpdate(k)
			}
		}
	}
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
