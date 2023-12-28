package autobuy

import (
	"context"
	"fmt"
	"sync"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

const ID = "autobuy"

var log = logrus.WithField("strategy", ID)

var two = fixedpoint.NewFromInt(2)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment
	Market      types.Market

	Symbol    string                  `json:"symbol"`
	Schedule  string                  `json:"schedule"`
	Threshold fixedpoint.Value        `json:"threshold"`
	PriceType PriceType               `json:"priceType"`
	Bollinger *types.BollingerSetting `json:"bollinger"`
	DryRun    bool                    `json:"dryRun"`

	bbgo.QuantityOrAmount

	boll *indicatorv2.BOLLStream
	cron *cron.Cron
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
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
	if err := s.QuantityOrAmount.Validate(); err != nil {
		return err
	}
	return nil
}

func (s *Strategy) Defaults() error {
	if s.PriceType == "" {
		s.PriceType = PriceTypeBuy
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Bollinger.Interval})
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	s.boll = session.Indicators(s.Symbol).BOLL(s.Bollinger.IntervalWindow, s.Bollinger.BandWidth)

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

	balance, ok := s.Session.GetAccount().Balance(s.Market.BaseCurrency)
	if !ok {
		log.Errorf("%s balance not found", s.Market.BaseCurrency)
		return
	}
	log.Infof("balance: %s", balance.String())

	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		log.WithError(err).Errorf("failed to query ticker")
		return
	}

	var price fixedpoint.Value
	switch s.PriceType {
	case PriceTypeLast:
		price = ticker.Last
	case PriceTypeBuy:
		price = ticker.Buy
	case PriceTypeSell:
		price = ticker.Sell
	case PriceTypeMid:
		price = ticker.Buy.Add(ticker.Sell).Div(two)
	default:
		price = ticker.Buy
	}

	if price.Float64() > s.boll.UpBand.Last(0) {
		log.Infof("price %s is higher than upper band %f, skip", price.String(), s.boll.UpBand.Last(0))
		return
	}

	if balance.Available.Compare(s.Threshold) > 0 {
		log.Infof("balance %s is higher than threshold %s", balance.Available.String(), s.Threshold.String())
		return
	}
	log.Infof("balance %s is lower than threshold %s", balance.Available.String(), s.Threshold.String())

	quantity := s.CalculateQuantity(price)
	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimitMaker,
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
