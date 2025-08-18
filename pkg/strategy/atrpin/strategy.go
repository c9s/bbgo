package atrpin

import (
	"context"
	"fmt"
	"sync"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

const ID = "atrpin"

var log = logrus.WithField("strategy", ID)

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

	// handle missing trades, will be removed in the future
	TakeProfitByExpectedBaseBalance bool             `json:"takeProfitByExpectedBaseBalance"`
	ExpectedBaseBalance             fixedpoint.Value `json:"expectedBaseBalance"`

	bbgo.QuantityOrAmount
	// bbgo.OpenPositionOptions

	logger *logrus.Entry
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}

	s.logger = log.WithFields(logrus.Fields{
		"symbol": s.Symbol,
		"window": s.Window,
	})
	return nil
}

func (s *Strategy) Validate() error {
	if s.ExpectedBaseBalance.Sign() < 0 {
		return fmt.Errorf("expectedBaseBalance should be non-negative")
	}
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
			s.logger.WithError(err).Error("unable to cancel open orders...")
			return
		}

		account, err := session.UpdateAccount(ctx)
		if err != nil {
			s.logger.WithError(err).Error("unable to update account")
			return
		}

		baseBalance, ok := account.Balance(s.Market.BaseCurrency)
		if !ok {
			s.logger.Errorf("%s balance not found", s.Market.BaseCurrency)
			return
		}
		quoteBalance, ok := account.Balance(s.Market.QuoteCurrency)
		if !ok {
			s.logger.Errorf("%s balance not found", s.Market.QuoteCurrency)
			return
		}

		lastAtr := atr.Last(0)
		s.logger.Infof("atr: %f", lastAtr)

		// protection
		if lastAtr <= k.High.Sub(k.Low).Float64() {
			lastAtr = k.High.Sub(k.Low).Float64()
		}

		priceRange := fixedpoint.NewFromFloat(lastAtr * s.Multiplier)

		// if the atr is too small, apply the price range protection with 10%
		// priceRange protection 10%
		priceRange = fixedpoint.Max(priceRange, k.Close.Mul(s.MinPriceRange))
		s.logger.Infof("priceRange: %f", priceRange.Float64())

		ticker, err := session.Exchange.QueryTicker(ctx, s.Symbol)
		if err != nil {
			s.logger.WithError(err).Error("unable to query ticker")
			return
		}

		s.logger.Info(ticker.String())

		bidPrice := fixedpoint.Max(ticker.Buy.Sub(priceRange), s.Market.TickSize)
		askPrice := ticker.Sell.Add(priceRange)

		bidQuantity := s.QuantityOrAmount.CalculateQuantity(bidPrice)
		askQuantity := s.QuantityOrAmount.CalculateQuantity(askPrice)

		var orderForms []types.SubmitOrder

		position := s.Strategy.OrderExecutor.Position()
		s.logger.Infof("position: %+v", position)

		base := position.GetBase()
		if s.TakeProfitByExpectedBaseBalance {
			base = baseBalance.Available.Sub(s.ExpectedBaseBalance)
		}

		side := types.SideTypeSell
		takerPrice := ticker.Buy
		if base.Sign() < 0 {
			side = types.SideTypeBuy
			takerPrice = ticker.Sell
		}

		positionQuantity := base.Abs()
		if !s.Market.IsDustQuantity(positionQuantity, takerPrice) {
			s.logger.Infof("%s position is not dust", s.Symbol)

			orderForms = append(orderForms, types.SubmitOrder{
				Symbol:      s.Symbol,
				Type:        types.OrderTypeLimit,
				Side:        side,
				Price:       takerPrice,
				Quantity:    positionQuantity,
				Market:      s.Market,
				TimeInForce: types.TimeInForceGTC,
				Tag:         "takeProfit",
			})

			s.logger.Infof("SUBMIT TAKER ORDER: %+v", orderForms)

			if _, err := s.Strategy.OrderExecutor.SubmitOrders(ctx, orderForms...); err != nil {
				s.logger.WithError(err).Errorf("unable to submit orders: %+v", orderForms)
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
			s.logger.Infof("no %s order to place", s.Symbol)
			return
		}

		s.logger.Infof("%s bid/ask: %f/%f", s.Symbol, bidPrice.Float64(), askPrice.Float64())

		s.logger.Infof("submit orders: %+v", orderForms)
		if _, err := s.Strategy.OrderExecutor.SubmitOrders(ctx, orderForms...); err != nil {
			s.logger.WithError(err).Errorf("unable to submit orders: %+v", orderForms)
		}
	}))

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if err := s.Strategy.OrderExecutor.GracefulCancel(ctx); err != nil {
			s.logger.WithError(err).Error("unable to cancel open orders...")
		}

		bbgo.Sync(ctx, s)
	})

	return nil
}
