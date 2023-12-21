package xfixedmaker

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/strategy/fixedmaker"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xfixedmaker"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// Fixed spread market making strategy
type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment

	TradingExchange string           `json:"tradingExchange"`
	Symbol          string           `json:"symbol"`
	Interval        types.Interval   `json:"interval"`
	Quantity        fixedpoint.Value `json:"quantity"`
	HalfSpread      fixedpoint.Value `json:"halfSpread"`
	OrderType       types.OrderType  `json:"orderType"`
	DryRun          bool             `json:"dryRun"`

	ReferenceExchange       string                   `json:"referenceExchange"`
	ReferencePriceEMA       types.IntervalWindow     `json:"referencePriceEMA"`
	OrderPriceLossThreshold fixedpoint.Value         `json:"orderPriceLossThreshold"`
	InventorySkew           fixedmaker.InventorySkew `json:"inventorySkew"`

	market                types.Market
	activeOrderBook       *bbgo.ActiveOrderBook
	orderPriceRiskControl *OrderPriceRiskControl
}

func (s *Strategy) Defaults() error {
	if s.OrderType == "" {
		log.Infof("order type is not set, using limit maker order type")
		s.OrderType = types.OrderTypeLimitMaker
	}
	return nil
}

func (s *Strategy) Initialize() error {
	s.Strategy = &common.Strategy{}
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Validate() error {
	if s.Quantity.Float64() <= 0 {
		return fmt.Errorf("quantity should be positive")
	}

	if s.HalfSpread.Float64() <= 0 {
		return fmt.Errorf("halfSpread should be positive")
	}

	if err := s.InventorySkew.Validate(); err != nil {
		return err
	}
	return nil
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	tradingSession, ok := sessions[s.TradingExchange]
	if !ok {
		log.Errorf("trading session %s is not defined", s.TradingExchange)
		return
	}
	tradingSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	if !s.CircuitBreakLossThreshold.IsZero() {
		tradingSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.CircuitBreakEMA.Interval})
	}

	referenceSession, ok := sessions[s.ReferenceExchange]
	if !ok {
		log.Errorf("reference session %s is not defined", s.ReferenceExchange)
	}
	referenceSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.ReferencePriceEMA.Interval})
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	tradingSession, ok := sessions[s.TradingExchange]
	if !ok {
		return fmt.Errorf("trading session %s is not defined", s.TradingExchange)
	}

	referenceSession, ok := sessions[s.ReferenceExchange]
	if !ok {
		return fmt.Errorf("reference session %s is not defined", s.ReferenceExchange)
	}

	market, ok := tradingSession.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("market %s not found", s.Symbol)
	}
	s.market = market

	s.Strategy.Initialize(ctx, s.Environment, tradingSession, s.market, ID, s.InstanceID())

	s.orderPriceRiskControl = NewOrderPriceRiskControl(
		referenceSession.Indicators(s.Symbol).EMA(s.ReferencePriceEMA),
		s.OrderPriceLossThreshold,
	)

	s.activeOrderBook = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeOrderBook.BindStream(tradingSession.UserDataStream)
	s.activeOrderBook.OnFilled(func(order types.Order) {
		if s.IsHalted(order.UpdateTime.Time()) {
			log.Infof("circuit break halted")
			return
		}

		if s.activeOrderBook.NumOfOrders() == 0 {
			log.Infof("no active orders, placing orders...")
			s.placeOrders(ctx)
		}
	})

	tradingSession.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		log.Infof("kline: %s", kline.String())

		if s.IsHalted(kline.EndTime.Time()) {
			log.Infof("circuit break halted")
			return
		}

		if kline.Interval == s.Interval {
			s.cancelOrders(ctx)
			s.placeOrders(ctx)
		}
	})

	// the shutdown handler, you can cancel all orders
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		_ = s.OrderExecutor.GracefulCancel(ctx)
	})
	return nil
}

func (s *Strategy) cancelOrders(ctx context.Context) {
	if err := s.activeOrderBook.GracefulCancel(ctx, s.Session.Exchange); err != nil {
		log.WithError(err).Errorf("failed to cancel orders")
	}
}

func (s *Strategy) placeOrders(ctx context.Context) {
	submitOrders, err := s.generateOrders(ctx)
	if err != nil {
		log.WithError(err).Error("failed to generate orders")
		return
	}
	log.Infof("submit orders: %+v", submitOrders)

	if s.DryRun {
		log.Infof("dry run, not submitting orders")
		return
	}

	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Error("failed to submit orders")
		return
	}
	log.Infof("created orders: %+v", createdOrders)

	s.activeOrderBook.Add(createdOrders...)
}

func (s *Strategy) generateOrders(ctx context.Context) ([]types.SubmitOrder, error) {
	orders := []types.SubmitOrder{}

	baseBalance, ok := s.Session.GetAccount().Balance(s.market.BaseCurrency)
	if !ok {
		return nil, fmt.Errorf("base currency %s balance not found", s.market.BaseCurrency)
	}
	log.Infof("base balance: %s", baseBalance.String())

	quoteBalance, ok := s.Session.GetAccount().Balance(s.market.QuoteCurrency)
	if !ok {
		return nil, fmt.Errorf("quote currency %s balance not found", s.market.QuoteCurrency)
	}
	log.Infof("quote balance: %s", quoteBalance.String())

	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		return nil, err
	}
	midPrice := ticker.Buy.Add(ticker.Sell).Div(fixedpoint.NewFromFloat(2.0))
	log.Infof("mid price: %s", midPrice.String())

	// calculate bid and ask price
	// sell price = mid price * (1 + r))
	// buy price = mid price * (1 - r))
	sellPrice := midPrice.Mul(fixedpoint.One.Add(s.HalfSpread)).Round(s.market.PricePrecision, fixedpoint.Up)
	buyPrice := midPrice.Mul(fixedpoint.One.Sub(s.HalfSpread)).Round(s.market.PricePrecision, fixedpoint.Down)
	log.Infof("sell price: %s, buy price: %s", sellPrice.String(), buyPrice.String())

	buyQuantity := s.Quantity
	sellQuantity := s.Quantity
	if !s.InventorySkew.InventoryRangeMultiplier.IsZero() {
		ratios := s.InventorySkew.CalculateBidAskRatios(
			s.Quantity,
			midPrice,
			baseBalance.Total(),
			quoteBalance.Total(),
		)
		log.Infof("bid ratio: %s, ask ratio: %s", ratios.BidRatio.String(), ratios.AskRatio.String())
		buyQuantity = s.Quantity.Mul(ratios.BidRatio)
		sellQuantity = s.Quantity.Mul(ratios.AskRatio)
		log.Infof("buy quantity: %s, sell quantity: %s", buyQuantity.String(), sellQuantity.String())
	}

	// check balance and generate orders
	amount := s.Quantity.Mul(buyPrice)
	if quoteBalance.Available.Compare(amount) > 0 {
		if s.orderPriceRiskControl.IsSafe(types.SideTypeBuy, buyPrice, s.Quantity) {
			orders = append(orders, types.SubmitOrder{
				Symbol:   s.Symbol,
				Side:     types.SideTypeBuy,
				Type:     s.OrderType,
				Price:    buyPrice,
				Quantity: buyQuantity,
			})

		} else {
			log.Infof("ref price risk control triggered, not placing buy order")
		}
	} else {
		log.Infof("not enough quote balance to buy, available: %s, amount: %s", quoteBalance.Available, amount)
	}

	if baseBalance.Available.Compare(s.Quantity) > 0 {
		if s.orderPriceRiskControl.IsSafe(types.SideTypeSell, sellPrice, s.Quantity) {
			orders = append(orders, types.SubmitOrder{
				Symbol:   s.Symbol,
				Side:     types.SideTypeSell,
				Type:     s.OrderType,
				Price:    sellPrice,
				Quantity: sellQuantity,
			})
		} else {
			log.Infof("ref price risk control triggered, not placing sell order")
		}
	} else {
		log.Infof("not enough base balance to sell, available: %s, quantity: %s", baseBalance.Available, s.Quantity)
	}

	return orders, nil
}
