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

	Interval      types.Interval   `json:"interval"`
	Window        int              `json:"window"`
	Multiplier    float64          `json:"multiplier"`
	MinPriceRange fixedpoint.Value `json:"minPriceRange"`

	bbgo.QuantityOrAmount
	// bbgo.OpenPositionOptions
}

func (s *Strategy) Initialize() error {
	s.Strategy = &common.Strategy{}
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s:%s:%d", ID, s.Symbol, s.Interval, s.Window)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
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
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	atr := session.Indicators(s.Symbol).ATR(s.Interval, s.Window)

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(k types.KLine) {
		if err := s.Strategy.OrderExecutor.GracefulCancel(ctx); err != nil {
			log.WithError(err).Error("unable to cancel open orders...")
		}

		account, err := session.UpdateAccount(ctx)
		if err != nil {
			log.WithError(err).Error("unable to update account")
			return
		}

		baseBalance, _ := account.Balance(s.Market.BaseCurrency)
		quoteBalance, _ := account.Balance(s.Market.QuoteCurrency)

		lastAtr := atr.Last(0)
		log.Infof("atr: %f", lastAtr)

		// protection
		if lastAtr <= k.High.Sub(k.Low).Float64() {
			lastAtr = k.High.Sub(k.Low).Float64()
		}

		priceRange := fixedpoint.NewFromFloat(lastAtr * s.Multiplier)

		// if the atr is too small, apply the price range protection with 10%
		// priceRange protection 10%
		priceRange = fixedpoint.Max(priceRange, k.Close.Mul(s.MinPriceRange))
		log.Infof("priceRange: %f", priceRange.Float64())

		ticker, err := session.Exchange.QueryTicker(ctx, s.Symbol)
		if err != nil {
			log.WithError(err).Error("unable to query ticker")
			return
		}

		log.Info(ticker.String())

		bidPrice := fixedpoint.Max(ticker.Buy.Sub(priceRange), s.Market.TickSize)
		askPrice := ticker.Sell.Add(priceRange)

		bidQuantity := s.QuantityOrAmount.CalculateQuantity(bidPrice)
		askQuantity := s.QuantityOrAmount.CalculateQuantity(askPrice)

		var orderForms []types.SubmitOrder

		position := s.Strategy.OrderExecutor.Position()
		if !position.IsDust() {
			log.Infof("position: %+v", position)

			side := types.SideTypeSell
			takerPrice := fixedpoint.Zero

			if position.IsShort() {
				side = types.SideTypeBuy
				takerPrice = ticker.Sell
			} else if position.IsLong() {
				side = types.SideTypeSell
				takerPrice = ticker.Buy
			}

			orderForms = append(orderForms, types.SubmitOrder{
				Symbol:      s.Symbol,
				Type:        types.OrderTypeLimit,
				Side:        side,
				Price:       takerPrice,
				Quantity:    position.GetQuantity(),
				Market:      s.Market,
				TimeInForce: types.TimeInForceGTC,
				Tag:         "takeProfit",
			})

			log.Infof("SUBMIT TAKER ORDER: %+v", orderForms)

			if _, err := s.Strategy.OrderExecutor.SubmitOrders(ctx, orderForms...); err != nil {
				log.WithError(err).Error("unable to submit orders")
			}

			return
		}

		askQuantity = s.Market.AdjustQuantityByMinNotional(askQuantity, askPrice)
		if !s.Market.IsDustQuantity(askQuantity, askPrice) && askQuantity.Compare(baseBalance.Available) < 0 {
			orderForms = append(orderForms, types.SubmitOrder{
				Symbol:      s.Symbol,
				Side:        types.SideTypeSell,
				Type:        types.OrderTypeLimitMaker,
				Quantity:    askQuantity,
				Price:       askPrice,
				Market:      s.Market,
				TimeInForce: types.TimeInForceGTC,
				Tag:         "pinOrder",
			})
		}

		bidQuantity = s.Market.AdjustQuantityByMinNotional(bidQuantity, bidPrice)
		if !s.Market.IsDustQuantity(bidQuantity, bidPrice) && bidQuantity.Mul(bidPrice).Compare(quoteBalance.Available) < 0 {
			orderForms = append(orderForms, types.SubmitOrder{
				Symbol:   s.Symbol,
				Side:     types.SideTypeBuy,
				Type:     types.OrderTypeLimitMaker,
				Price:    bidPrice,
				Quantity: bidQuantity,
				Market:   s.Market,
				Tag:      "pinOrder",
			})
		}

		if len(orderForms) == 0 {
			log.Infof("no order to place")
			return
		}

		log.Infof("bid/ask: %f/%f", bidPrice.Float64(), askPrice.Float64())

		if _, err := s.Strategy.OrderExecutor.SubmitOrders(ctx, orderForms...); err != nil {
			log.WithError(err).Error("unable to submit orders")
		}
	}))

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if err := s.Strategy.OrderExecutor.GracefulCancel(ctx); err != nil {
			log.WithError(err).Error("unable to cancel open orders...")
		}
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
