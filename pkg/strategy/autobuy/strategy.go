package autobuy

import (
	"context"
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "autobuy"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment
	Market      types.Market

	Symbol         string                  `json:"symbol"`
	Schedule       string                  `json:"schedule"`
	MinBaseBalance fixedpoint.Value        `json:"minBaseBalance"`
	OrderType      types.OrderType         `json:"orderType"`
	PriceType      types.PriceType         `json:"priceType"`
	Bollinger      *types.BollingerSetting `json:"bollinger"`
	DryRun         bool                    `json:"dryRun"`

	// Deprecated, to be replaced by MinBaseBalance
	Threshold fixedpoint.Value `json:"threshold"`

	bbgo.QuantityOrAmount

	boll *indicatorv2.BOLLStream
	cron *cron.Cron
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}

	if s.Threshold.Sign() > 0 && s.MinBaseBalance.IsZero() {
		s.MinBaseBalance = s.Threshold
	}
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Validate() error {
	if s.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}

	if s.Schedule == "" {
		return fmt.Errorf("schedule is required")
	}

	if s.MinBaseBalance.Sign() <= 0 {
		return fmt.Errorf("minBaseBalance must be greater than 0")
	}

	if err := s.QuantityOrAmount.Validate(); err != nil {
		return err
	}

	if s.Bollinger != nil {
		if s.Bollinger.Interval == "" {
			return fmt.Errorf("bollinger interval is required")
		}

		if s.Bollinger.BandWidth <= 0 {
			return fmt.Errorf("bollinger band width must be greater than 0")
		}
	}
	return nil
}

func (s *Strategy) Defaults() error {
	if s.OrderType == "" {
		s.OrderType = types.OrderTypeLimit
	}

	if s.PriceType == "" {
		s.PriceType = types.PriceTypeMaker
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	if s.Bollinger != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Bollinger.Interval})
	}
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	if s.Bollinger != nil {
		s.boll = session.Indicators(s.Symbol).BOLL(s.Bollinger.IntervalWindow, s.Bollinger.BandWidth)
	}

	s.OrderExecutor.ActiveMakerOrders().OnFilled(func(order types.Order) {
		s.autobuy(ctx)
	})

	// the shutdown handler, you can cancel all orders
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		s.cancelOrders(ctx)
	})

	s.cron = cron.New()
	s.cron.AddFunc(s.Schedule, func() {
		s.autobuy(ctx)
	})
	s.cron.Start()

	return nil
}

func (s *Strategy) cancelOrders(ctx context.Context) {
	if err := s.OrderExecutor.GracefulCancel(ctx); err != nil {
		log.WithError(err).Errorf("failed to cancel orders")
	}
}

func (s *Strategy) autobuy(ctx context.Context) {
	s.cancelOrders(ctx)

	baseBalance, ok := s.Session.GetAccount().Balance(s.Market.BaseCurrency)
	if !ok {
		log.Errorf("%s balance not found", s.Market.BaseCurrency)
		return
	}
	log.Infof("balance: %s", baseBalance.String())

	quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
	if !ok {
		log.Errorf("%s balance not found", s.Market.QuoteCurrency)
		return
	}

	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		log.WithError(err).Errorf("failed to query ticker")
		return
	}

	side := types.SideTypeBuy
	price := s.PriceType.GetPrice(ticker, side)

	if s.boll != nil && price.Float64() > s.boll.UpBand.Last(0) {
		log.Infof("price %s is higher than upper band %f, skip", price.String(), s.boll.UpBand.Last(0))
		return
	}

	if baseBalance.Available.Compare(s.MinBaseBalance) > 0 {
		log.Infof("balance %s is higher than minBaseBalance %s", baseBalance.Available.String(), s.MinBaseBalance.String())
		return
	}
	log.Infof("balance %s is lower than minBaseBalance %s", baseBalance.Available.String(), s.MinBaseBalance.String())

	quantity := s.CalculateQuantity(price)

	requiredQuote := quantity.Mul(price)
	if quoteBalance.Available.Compare(requiredQuote) < 0 {
		log.Warnf("quote balance %s is not enough: %s < %s", s.Market.QuoteCurrency, quoteBalance.Available.String(), requiredQuote.String())
		return
	}

	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     side,
		Type:     s.OrderType,
		Quantity: quantity,
		Price:    price,
	}

	if s.DryRun {
		log.Infof("dry run, skip")
		return
	}

	log.Infof("submitting order: %s", submitOrder.String())
	_, err = s.OrderExecutor.SubmitOrders(ctx, submitOrder)
	if err != nil {
		log.WithError(err).Errorf("submit order error")
	}
}
