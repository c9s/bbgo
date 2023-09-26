package atrpin

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "atrpin"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment
	Market      types.Market

	Symbol string `json:"symbol"`

	Interval   types.Interval `json:"interval"`
	Window     int            `json:"slowWindow"`
	Multiplier float64        `json:"multiplier"`

	bbgo.QuantityOrAmount
	// bbgo.OpenPositionOptions
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s:%s:%d", ID, s.Symbol, s.Interval, s.Window)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Defaults() error {
	if s.Multiplier == 0.0 {
		s.Multiplier = 10.0
	}

	if s.Interval == "" {
		s.Interval = types.Interval5m
	}

	return nil
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy = &common.Strategy{}
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	atr := session.Indicators(s.Symbol).ATR(s.Interval, s.Window)
	session.UserDataStream.OnKLine(types.KLineWith(s.Symbol, s.Interval, func(k types.KLine) {
		if err := s.Strategy.OrderExecutor.GracefulCancel(ctx); err != nil {
			log.WithError(err).Error("unable to cancel open orders...")
		}

		lastAtr := atr.Last(0)

		// protection
		if lastAtr <= k.High.Sub(k.Low).Float64() {
			lastAtr = k.High.Sub(k.Low).Float64()
		}

		priceRange := fixedpoint.NewFromFloat(lastAtr * s.Multiplier)

		ticker, err := session.Exchange.QueryTicker(ctx, s.Symbol)
		if err != nil {
			log.WithError(err).Error("unable to query ticker")
			return
		}

		bidPrice := ticker.Buy.Sub(priceRange)
		askPrice := ticker.Sell.Add(priceRange)

		bidQuantity := s.QuantityOrAmount.CalculateQuantity(bidPrice)
		askQuantity := s.QuantityOrAmount.CalculateQuantity(askPrice)

		var orderForms []types.SubmitOrder

		position := s.Strategy.OrderExecutor.Position()
		if !position.IsDust() {
			side := types.SideTypeSell
			takerPrice := fixedpoint.Zero

			if position.IsShort() {
				side = types.SideTypeBuy
				takerPrice = askPrice
			} else if position.IsLong() {
				side = types.SideTypeSell
				takerPrice = bidPrice
			}

			orderForms = append(orderForms, types.SubmitOrder{
				Symbol:   s.Symbol,
				Side:     side,
				Price:    takerPrice,
				Quantity: position.GetQuantity(),
				Market:   s.Market,
			})
		}

		orderForms = append(orderForms, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Price:    askPrice,
			Quantity: askQuantity,
			Market:   s.Market,
		})

		orderForms = append(orderForms, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Price:    bidPrice,
			Quantity: bidQuantity,
			Market:   s.Market,
		})

		if _, err := s.Strategy.OrderExecutor.SubmitOrders(ctx, orderForms...); err != nil {
			log.WithError(err).Error("unable to submit orders")
		}
	}))

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
