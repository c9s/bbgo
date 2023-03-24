package neutralmaker

import (
	"context"
	"fmt"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/sirupsen/logrus"
	"math"
	"sync"
	"time"
)

const ID = "neutralmaker"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// Fixed spread market making strategy
type Strategy struct {
	Environment *bbgo.Environment

	Symbol        string           `json:"symbol"`
	LotSize       fixedpoint.Value `json:"lotSize"`
	PositionLimit fixedpoint.Value `json:"positionLimit"`
	HalfSpread    fixedpoint.Value `json:"halfSpread"`
	OrderType     types.OrderType  `json:"orderType"`
	DryRun        bool             `json:"dryRun"`

	// SourceExchange session name
	SpotExchange string `json:"spotExchange"`
	// MakerExchange session name
	FutureExchange string `json:"futureExchange"`

	SpotSession   *bbgo.ExchangeSession
	FutureSession *bbgo.ExchangeSession
	SpotMarket    types.Market
	FutureMarket  types.Market

	BestBidPrice fixedpoint.Value
	BestAskPrice fixedpoint.Value

	activeMakerOrders *bbgo.ActiveOrderBook

	// persistence fields
	SpotPosition      *types.Position    `json:"position,omitempty" persistence:"position"`
	SpotProfitStats   *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`
	FuturePosition    *types.Position    `json:"position,omitempty" persistence:"future_position"`
	FutureProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"future_profit_stats"`

	spotStreamBook   *types.StreamOrderBook
	spot2StreamBook  *types.StreamOrderBook
	futureStreamBook *types.StreamOrderBook

	SpotOrderExecutor   *bbgo.GeneralOrderExecutor
	FutureOrderExecutor *bbgo.GeneralOrderExecutor
}

func (s *Strategy) Defaults() error {
	if s.OrderType == "" {
		s.OrderType = types.OrderTypeLimitMaker
	}
	return nil
}
func (s *Strategy) Initialize() error {
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Validate() error {
	if s.LotSize.Float64() <= 0 {
		return fmt.Errorf("quantity should be positive")
	}
	return nil
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {

	log.Warnf("%+v\t%+v", s.SpotExchange, s.FutureExchange)

	s.SpotSession = sessions[s.SpotExchange]
	//if !sok {
	//	fmt.Errorf("spot session %s is not defined", s.SpotExchange)
	//}
	s.SpotSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
	s.SpotSession.Subscribe(types.BookChannel, "BTCBUSD", types.SubscribeOptions{})

	s.FutureSession = sessions[s.FutureExchange]
	//if !fok {
	//	fmt.Errorf("future session %s is not defined", s.FutureExchange)
	//}
	s.FutureSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
}

func (s *Strategy) CrossRun(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	// configure sessions
	spotSession, ok := sessions[s.SpotExchange]
	if !ok {
		return fmt.Errorf("spot exchange session %s is not defined", s.SpotExchange)
	}

	s.SpotSession = spotSession

	//futureSession, ok := sessions[s.FutureExchange]
	//if !ok {
	//	return fmt.Errorf("future exchange session %s is not defined", s.FutureExchange)
	//}
	//
	//s.futureSession = futureSessionspotMarket
	log.Errorf("%+v", s.FutureSession.Futures)

	s.SpotMarket, ok = s.SpotSession.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("spot session market %s is not defined", s.Symbol)
	}

	s.FutureMarket, ok = s.FutureSession.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("future session market %s is not defined", s.Symbol)
	}

	s.activeMakerOrders = bbgo.NewActiveOrderBook(s.SpotMarket.Symbol)
	s.activeMakerOrders.BindStream(s.SpotSession.UserDataStream)

	instanceID := s.InstanceID()

	if s.SpotPosition == nil {
		s.SpotPosition = types.NewPositionFromMarket(s.SpotMarket)
	}
	if s.FuturePosition == nil {
		s.FuturePosition = types.NewPositionFromMarket(s.FutureMarket)
	}

	// Always update the position fields
	s.SpotPosition.Strategy = ID
	s.SpotPosition.StrategyInstanceID = instanceID
	s.FuturePosition.Strategy = ID
	s.FuturePosition.StrategyInstanceID = instanceID

	if s.SpotProfitStats == nil {
		s.SpotProfitStats = types.NewProfitStats(s.SpotMarket)
	}

	if s.SpotOrderExecutor == nil {
		s.SpotOrderExecutor = bbgo.NewGeneralOrderExecutor(s.SpotSession, s.SpotMarket.Symbol, s.ID(), s.InstanceID(), s.SpotPosition)
	}
	s.SpotOrderExecutor.BindProfitStats(s.SpotProfitStats)
	s.SpotOrderExecutor.Bind()
	s.SpotOrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})

	if s.FutureOrderExecutor == nil {
		s.FutureOrderExecutor = bbgo.NewGeneralOrderExecutor(s.FutureSession, s.FutureMarket.Symbol, s.ID(), s.InstanceID(), s.FuturePosition)
	}
	//s.FutureOrderExecutor.BindProfitStats(s.FutureProfitStats)
	s.FutureOrderExecutor.Bind()
	s.FutureOrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})

	s.activeMakerOrders.OnFilled(func(order types.Order) {
		log.Infof("active orders filled, hedge")
		if order.Side == types.SideTypeBuy {
			s.hedge(ctx, orderExecutionRouter, order.ExecutedQuantity, types.SideTypeSell)
		} else if order.Side == types.SideTypeSell {
			s.hedge(ctx, orderExecutionRouter, order.ExecutedQuantity, types.SideTypeBuy)
		}
	})

	s.futureStreamBook = types.NewStreamBook(s.FutureMarket.Symbol)
	s.futureStreamBook.BindStream(s.FutureSession.MarketDataStream)
	s.spotStreamBook = types.NewStreamBook(s.SpotMarket.Symbol)
	s.spotStreamBook.BindStream(s.SpotSession.MarketDataStream)
	s.spot2StreamBook = types.NewStreamBook("BTCBUSD")
	s.spot2StreamBook.BindStream(s.SpotSession.MarketDataStream)

	go func() {
		posTicker := time.NewTicker(util.MillisecondsJitter(types.Interval("1000ms").Duration(), 200))
		defer posTicker.Stop()
		for {
			select {

			case <-ctx.Done():
				log.Warnf("%s maker goroutine stopped, due to the cancelled context", s.Symbol)
				return
			case <-posTicker.C:
				s.cancelOrders(ctx)
				sbid, sbok := s.spotStreamBook.OrderBook.BestBid()
				sask, saok := s.spotStreamBook.OrderBook.BestAsk()
				s2bid, s2bok := s.spot2StreamBook.OrderBook.BestBid()
				s2ask, s2aok := s.spot2StreamBook.OrderBook.BestAsk()
				fbid, fbok := s.futureStreamBook.BestBid()
				fask, faok := s.futureStreamBook.BestAsk()
				log.Infof("Futures Bid Price: %f, Future Ask Price: %f\n Spot Bid Price: %f, Spot Ask Price: %f", fbid.Price.Float64(), fask.Price.Float64(), sbid.Price.Float64(), sask.Price.Float64())
				if fbok && faok && sbok && saok && s2bok && s2aok {
					s.replenish(ctx, orderExecutionRouter, fbid.Price, fask.Price, sbid.Price, sask.Price, s2bid.Price, s2ask.Price)
				}
			}
		}
	}()

	// the shutdown handler, you can cancel all orders
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		_ = orderExecutionRouter.CancelOrdersTo(ctx, s.SpotExchange) //.GracefulCancel(ctx)
		_ = orderExecutionRouter.CancelOrdersTo(ctx, s.FutureExchange)
	})

	return nil
}

func (s *Strategy) cancelOrders(ctx context.Context) {
	if err := s.SpotOrderExecutor.GracefulCancel(ctx); err != nil { //orderExecutionRouter.CancelOrdersTo(ctx, s.SpotExchange, s.activeMakerOrders.Orders()...); err != nil {
		log.WithError(err).Errorf("failed to cancel orders")
	}
}

func (s *Strategy) replenish(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, futBidPrice, futAskPrice, spotBidPrice, spotAskPrice, spot2BidPrice, spot2AskPrice fixedpoint.Value) {
	submitOrders, err := s.generateSubmitOrders(ctx, futBidPrice, futAskPrice, spotBidPrice, spotAskPrice, spot2BidPrice, spot2AskPrice)
	if err != nil {
		log.WithError(err).Error("failed to generate submit orders")
		return
	}
	log.Infof("submit orders: %+v", submitOrders)

	if s.DryRun {
		log.Infof("dry run, not submitting orders")
		return

	}

	createdOrders, err := s.SpotOrderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Error("failed to submit orders")
		return
	}
	log.Infof("created orders: %+v", createdOrders)

	s.activeMakerOrders.Add(createdOrders...)
}

func (s *Strategy) generateSubmitOrders(ctx context.Context, futBidPrice, futAskPrice, spotBidPrice, spotAskPrice, spot2BidPrice, spot2AskPrice fixedpoint.Value) ([]types.SubmitOrder, error) {
	baseBalance, ok := s.SpotSession.GetAccount().Balance(s.SpotMarket.BaseCurrency)
	if !ok {
		return nil, fmt.Errorf("base currency %s balance not found", s.SpotMarket.BaseCurrency)
	}
	log.Infof("base balance: %+v", baseBalance)

	quoteBalance, ok := s.SpotSession.GetAccount().Balance(s.SpotMarket.QuoteCurrency)
	if !ok {
		return nil, fmt.Errorf("quote currency %s balance not found", s.SpotMarket.QuoteCurrency)
	}
	log.Infof("quote balance: %+v", quoteBalance)

	orders := []types.SubmitOrder{}

	// calculate buy and sell price
	buyPrice := futBidPrice.Sub(s.HalfSpread) //.Sub(fixedpoint.NewFromInt(2)) //.Mul(fixedpoint.One.Sub(s.HalfSpreadRatio))
	log.Infof("buy price: %+v", buyPrice)
	sellPrice := futAskPrice.Add(s.HalfSpread) //.Mul(fixedpoint.One.Add(s.HalfSpreadRatio))
	log.Infof("sell price: %+v", sellPrice)

	// check balance and generate orders
	position := s.SpotOrderExecutor.Position()
	buySize := fixedpoint.NewFromFloat(s.LotSize.Float64() * (1 - math.Min(position.Base.Float64(), s.PositionLimit.Float64())/s.PositionLimit.Float64()))
	sellSize := fixedpoint.NewFromFloat(s.LotSize.Float64() * (1 + math.Min(position.Base.Float64(), s.PositionLimit.Float64())/s.PositionLimit.Float64()))
	log.Info(s.LotSize, buySize, position.Base, s.PositionLimit)
	if buyPrice.Compare(spotAskPrice) < 0 && quoteBalance.Available.Compare(buySize.Mul(buyPrice)) > 0 && position.Base.Compare(s.PositionLimit) < 0 {
		orders = append(orders, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Type:     s.OrderType,
			Price:    buyPrice,
			Quantity: buySize, //.Div(buyPrice),
			Tag:      "NeedHedge",
		})
	} else {
		log.Infof("not enough quote balance to buy, available: %s, amount: %s", quoteBalance.Available, buySize.Mul(buyPrice))
	}

	if sellPrice.Compare(spotBidPrice) > 0 && baseBalance.Available.Compare(sellSize) > 0 && position.Base.Compare(s.PositionLimit.Neg()) > 0 {
		orders = append(orders, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     s.OrderType,
			Price:    sellPrice,
			Quantity: sellSize, //.Div(sellPrice),
			Tag:      "NeedHedge",
		})
	} else {
		log.Infof("not enough base balance to sell, available: %s, quantity: %s", baseBalance.Available, sellSize)
	}

	return orders, nil
}

func (s *Strategy) hedge(ctx context.Context, orderExecutionRoute bbgo.OrderExecutionRouter, volume fixedpoint.Value, side types.SideType) {
	submitOrders, err := s.generateHedgeOrders(ctx, volume, side)
	if err != nil {
		log.WithError(err).Error("failed to generate submit orders")
		return
	}
	log.Infof("submit orders: %+v", submitOrders)

	if s.DryRun {
		log.Infof("dry run, not submitting orders")
		return
	}

	createdOrders, err := s.FutureOrderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Error("failed to submit orders")
		return
	}
	log.Infof("created orders: %+v", createdOrders)

	//s.activeMakerOrders.Add(createdOrders...)
}

func (s *Strategy) generateHedgeOrders(ctx context.Context, volume fixedpoint.Value, side types.SideType) ([]types.SubmitOrder, error) {
	baseBalance, ok := s.FutureSession.GetAccount().Balance(s.FutureMarket.BaseCurrency)
	if !ok {
		return nil, fmt.Errorf("base currency %s balance not found", s.FutureMarket.BaseCurrency)
	}
	log.Infof("base balance: %+v", baseBalance)

	quoteBalance, ok := s.FutureSession.GetAccount().Balance(s.FutureMarket.QuoteCurrency)
	if !ok {
		return nil, fmt.Errorf("quote currency %s balance not found", s.FutureMarket.QuoteCurrency)
	}
	log.Infof("quote balance: %+v", quoteBalance)

	orders := []types.SubmitOrder{}

	if side == types.SideTypeBuy {
		orders = append(orders, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: volume,
			Tag:      "Neutralization",
		})
	} else {
		log.Infof("not enough quote balance to buy, available: %s, amount: %s", quoteBalance.Available, volume)
	}

	if side == types.SideTypeSell {
		orders = append(orders, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeMarket,
			Quantity: volume,
			Tag:      "Neutralization",
		})
	} else {
		log.Infof("not enough base balance to sell, available: %s, amount: %s", baseBalance.Available, volume)
	}

	return orders, nil
}
