package xmaker

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

var defaultMargin = fixedpoint.NewFromFloat(0.01)

var defaultQuantity = fixedpoint.NewFromFloat(0.001)

const ID = "xmaker"

const stateKey = "state-v1"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	HedgePosition     fixedpoint.Value `json:"hedgePosition"`
	Position          *bbgo.Position   `json:"position,omitempty"`
	AccumulatedVolume fixedpoint.Value `json:"accumulatedVolume,omitempty"`
	AccumulatedProfit fixedpoint.Value `json:"accumulatedProfit,omitempty"`
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence

	Symbol         string `json:"symbol"`
	SourceExchange string `json:"sourceExchange"`
	MakerExchange  string `json:"makerExchange"`

	UpdateInterval      types.Duration `json:"updateInterval"`
	HedgeInterval       types.Duration `json:"hedgeInterval"`
	OrderCancelWaitTime types.Duration `json:"orderCancelWaitTime"`

	Margin    fixedpoint.Value `json:"margin"`
	BidMargin fixedpoint.Value `json:"bidMargin"`
	AskMargin fixedpoint.Value `json:"askMargin"`

	// Quantity is used for fixed quantity of the first layer
	Quantity fixedpoint.Value `json:"quantity"`

	// QuantityMultiplier is the factor that multiplies the quantity of the previous layer
	QuantityMultiplier fixedpoint.Value `json:"quantityMultiplier"`

	// QuantityScale helps user to define the quantity by layer scale
	QuantityScale *bbgo.LayerScale `json:"quantityScale,omitempty"`

	MaxExposurePosition fixedpoint.Value `json:"maxExposurePosition"`
	DisableHedge        bool             `json:"disableHedge"`

	NumLayers int `json:"numLayers"`
	Pips      int `json:"pips"`

	makerSession  *bbgo.ExchangeSession
	sourceSession *bbgo.ExchangeSession

	sourceMarket types.Market
	makerMarket  types.Market

	state *State

	book              *types.StreamOrderBook
	activeMakerOrders *bbgo.LocalActiveOrderBook

	orderStore *bbgo.OrderStore

	lastPrice float64
	groupID   uint32

	stopC chan struct{}
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	sourceSession, ok := sessions[s.SourceExchange]
	if !ok {
		panic(fmt.Errorf("source session %s is not defined", s.SourceExchange))
	}

	sourceSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})

	makerSession, ok := sessions[s.MakerExchange]
	if !ok {
		panic(fmt.Errorf("maker session %s is not defined", s.MakerExchange))
	}
	makerSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) updateQuote(ctx context.Context) {
	if err := s.makerSession.Exchange.CancelOrders(ctx, s.activeMakerOrders.Orders()...); err != nil {
		log.WithError(err).Errorf("can not cancel orders")
		return
	}

	// avoid unlock issue
	if s.OrderCancelWaitTime > 0 {
		time.Sleep(s.OrderCancelWaitTime.Duration())
	} else {
		// use the default wait time
		time.Sleep(500 * time.Millisecond)
	}

	if s.activeMakerOrders.NumOfAsks() > 0 || s.activeMakerOrders.NumOfBids() > 0 {
		log.Warnf("there are some %s orders not canceled, skipping placing maker orders", s.Symbol)
		s.activeMakerOrders.Print()
		return
	}

	sourceBook := s.book.Get()
	if len(sourceBook.Bids) == 0 || len(sourceBook.Asks) == 0 {
		return
	}

	if valid, err := sourceBook.IsValid(); !valid {
		log.WithError(err).Errorf("%s invalid order book: %v", s.Symbol, err)
		return
	}

	bestBidPrice := sourceBook.Bids[0].Price
	bestAskPrice := sourceBook.Asks[0].Price
	log.Infof("%s best bid price %f, best ask price: %f", s.Symbol, bestBidPrice.Float64(), bestAskPrice.Float64())

	bidPrice := bestBidPrice.MulFloat64(1.0 - s.BidMargin.Float64())

	askPrice := bestAskPrice.MulFloat64(1.0 + s.AskMargin.Float64())

	log.Infof("%s quote bid price: %f ask price: %f", s.Symbol, bidPrice.Float64(), askPrice.Float64())

	var disableMakerBid = false
	var disableMakerAsk = false
	var submitOrders []types.SubmitOrder

	// we load the balances from the account,
	// however, while we're generating the orders,
	// the balance may have a chance to be deducted by other strategies or manual orders submitted by the user
	makerBalances := s.makerSession.Account.Balances()
	makerQuota := &bbgo.QuotaTransaction{}
	if b, ok := makerBalances[s.makerMarket.BaseCurrency]; ok {
		makerQuota.BaseAsset.Add(b.Available)

		if b.Available.Float64() <= s.makerMarket.MinQuantity {
			disableMakerAsk = true
		}
	}

	if b, ok := makerBalances[s.makerMarket.QuoteCurrency]; ok {
		makerQuota.QuoteAsset.Add(b.Available)

		if b.Available.Float64() <= s.makerMarket.MinNotional {
			disableMakerBid = true
		}
	}

	// if max exposure position is configured, we should not:
	// 1. place bid orders when we already bought too much
	// 2. place ask orders when we already sold too much
	if s.MaxExposurePosition > 0 {
		pos := s.state.HedgePosition.AtomicLoad()
		if pos < -s.MaxExposurePosition {
			disableMakerAsk = true
		} else if pos > s.MaxExposurePosition {
			disableMakerBid = true
		}
	}

	hedgeBalances := s.sourceSession.Account.Balances()
	hedgeQuota := &bbgo.QuotaTransaction{}
	if b, ok := hedgeBalances[s.sourceMarket.BaseCurrency]; ok {
		hedgeQuota.BaseAsset.Add(b.Available)

		// to make bid orders, we need enough base asset in the foreign exchange,
		// if the base asset balance is not enough for selling
		if b.Available.Float64() <= s.sourceMarket.MinQuantity {
			disableMakerBid = true
		}
	}

	if b, ok := hedgeBalances[s.sourceMarket.QuoteCurrency]; ok {
		hedgeQuota.QuoteAsset.Add(b.Available)

		// to make ask orders, we need enough quote asset in the foreign exchange,
		// if the quote asset balance is not enough for buying
		if b.Available.Float64() <= s.sourceMarket.MinNotional {
			disableMakerAsk = true
		}
	}

	if disableMakerAsk && disableMakerBid {
		log.Warn("maker is disabled due to insufficient balances")
		return
	}

	bidQuantity := s.Quantity
	askQuantity := s.Quantity
	for i := 0; i < s.NumLayers; i++ {
		// for maker bid orders
		if !disableMakerBid {
			if s.QuantityScale != nil {
				qf, err := s.QuantityScale.Scale(i + 1)
				if err != nil {
					log.WithError(err).Errorf("quantityScale error")
					return
				}

				log.Infof("scaling quantity to %f by layer: %d", qf, i+1)

				// override the default bid quantity
				bidQuantity = fixedpoint.NewFromFloat(qf)
			}

			if makerQuota.QuoteAsset.Lock(bidQuantity.Mul(bidPrice)) && hedgeQuota.BaseAsset.Lock(bidQuantity) {
				// if we bought, then we need to sell the base from the hedge session
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Type:        types.OrderTypeLimit,
					Side:        types.SideTypeBuy,
					Price:       bidPrice.Float64(),
					Quantity:    bidQuantity.Float64(),
					TimeInForce: "GTC",
					GroupID:     s.groupID,
				})

				makerQuota.Commit()
				hedgeQuota.Commit()
			} else {
				makerQuota.Rollback()
				hedgeQuota.Rollback()
			}

			bidPrice -= fixedpoint.NewFromFloat(s.makerMarket.TickSize * float64(s.Pips))

			if s.QuantityMultiplier > 0 {
				bidQuantity = bidQuantity.Mul(s.QuantityMultiplier)
			}
		}

		// for maker ask orders
		if !disableMakerAsk {
			if s.QuantityScale != nil {
				qf, err := s.QuantityScale.Scale(i + 1)
				if err != nil {
					log.WithError(err).Errorf("quantityScale error")
					return
				}

				// override the default bid quantity
				askQuantity = fixedpoint.NewFromFloat(qf)
			}

			if makerQuota.BaseAsset.Lock(askQuantity) && hedgeQuota.QuoteAsset.Lock(askQuantity.Mul(askPrice)) {
				// if we bought, then we need to sell the base from the hedge session
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Type:        types.OrderTypeLimit,
					Side:        types.SideTypeSell,
					Price:       askPrice.Float64(),
					Quantity:    askQuantity.Float64(),
					TimeInForce: "GTC",
					GroupID:     s.groupID,
				})
				makerQuota.Commit()
				hedgeQuota.Commit()
			} else {
				makerQuota.Rollback()
				hedgeQuota.Rollback()
			}
			askPrice += fixedpoint.NewFromFloat(s.makerMarket.TickSize * float64(s.Pips))

			if s.QuantityMultiplier > 0 {
				askQuantity = askQuantity.Mul(s.QuantityMultiplier)
			}
		}
	}

	if len(submitOrders) == 0 {
		return
	}

	makerOrderExecutor := &bbgo.ExchangeOrderExecutor{Session: s.makerSession}
	makerOrders, err := makerOrderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Errorf("order error: %s", err.Error())
		return
	}

	s.activeMakerOrders.Add(makerOrders...)
	s.orderStore.Add(makerOrders...)
}

func (s *Strategy) Hedge(ctx context.Context, pos fixedpoint.Value) {
	side := types.SideTypeBuy

	if pos == 0 {
		return
	}

	quantity := pos
	if pos < 0 {
		side = types.SideTypeSell
		quantity = -pos
	}

	lastPrice := s.lastPrice
	sourceBook := s.book.Get()
	switch side {

	case types.SideTypeBuy:
		if len(sourceBook.Asks) > 0 {
			if pv, ok := sourceBook.Asks.First(); ok {
				lastPrice = pv.Price.Float64()
			}
		}

	case types.SideTypeSell:
		if len(sourceBook.Bids) > 0 {
			if pv, ok := sourceBook.Bids.First(); ok {
				lastPrice = pv.Price.Float64()
			}
		}

	}

	notional := quantity.MulFloat64(lastPrice)
	if notional.Float64() <= s.sourceMarket.MinNotional {
		log.Warnf("less than min notional %f, skipping", notional.Float64())
		return
	}

	s.Notifiability.Notify("Submitting hedge order: %s %s %f", s.Symbol, side, quantity.Float64())
	orderExecutor := &bbgo.ExchangeOrderExecutor{Session: s.sourceSession}
	returnOrders, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:   s.Symbol,
		Type:     types.OrderTypeMarket,
		Side:     side,
		Quantity: quantity.Float64(),
	})

	if err != nil {
		log.WithError(err).Errorf("market order submit error: %s", err.Error())
		return
	}

	s.orderStore.Add(returnOrders...)
}

func (s *Strategy) handleTradeUpdate(trade types.Trade) {
	log.Infof("received trade %+v", trade)

	if trade.Symbol != s.Symbol {
		return
	}

	if !s.orderStore.Exists(trade.OrderID) {
		return
	}

	q := fixedpoint.NewFromFloat(trade.Quantity)
	switch trade.Side {
	case types.SideTypeSell:
		q = -q

	case types.SideTypeBuy:

	case types.SideTypeSelf:
		// ignore self trades

	default:
		log.Infof("ignore non sell/buy side trades, got: %v", trade.Side)
		return

	}

	log.Infof("identified %s trade %d with an existing order: %d", trade.Symbol, trade.ID, trade.OrderID)

	s.state.HedgePosition.AtomicAdd(q)
	s.state.AccumulatedVolume.AtomicAdd(fixedpoint.NewFromFloat(trade.Quantity))

	if profit, madeProfit := s.state.Position.AddTrade(trade); madeProfit {
		s.state.AccumulatedProfit.AtomicAdd(profit)

		s.Notify("%s trade just made profit %f %s, accumulated profit %f %s", s.Symbol,
			profit.Float64(), s.state.Position.QuoteCurrency,
			s.state.AccumulatedProfit.Float64(), s.state.Position.QuoteCurrency)
	} else {
		s.Notify("%s trade modified the position: average cost = %f %s, base = %f", s.Symbol, s.state.Position.AverageCost.Float64(), s.state.Position.QuoteCurrency, s.state.Position.Base.Float64())
	}

	s.lastPrice = trade.Price
}

func (s *Strategy) Validate() error {
	if s.Quantity == 0 || s.QuantityScale == nil {
		return errors.New("quantity or quantityScale can not be empty")
	}

	if s.QuantityMultiplier != 0 && s.QuantityMultiplier < 0 {
		return errors.New("quantityMultiplier can not be a negative number")
	}

	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	// configure default values
	if s.UpdateInterval == 0 {
		s.UpdateInterval = types.Duration(time.Second)
	}

	if s.HedgeInterval == 0 {
		s.HedgeInterval = types.Duration(10 * time.Second)
	}

	if s.NumLayers == 0 {
		s.NumLayers = 1
	}

	if s.BidMargin == 0 {
		if s.Margin != 0 {
			s.BidMargin = s.Margin
		} else {
			s.BidMargin = defaultMargin
		}
	}

	if s.AskMargin == 0 {
		if s.Margin != 0 {
			s.AskMargin = s.Margin
		} else {
			s.AskMargin = defaultMargin
		}
	}

	// configure sessions
	sourceSession, ok := sessions[s.SourceExchange]
	if !ok {
		return fmt.Errorf("source exchange session %s is not defined", s.SourceExchange)
	}

	s.sourceSession = sourceSession

	makerSession, ok := sessions[s.MakerExchange]
	if !ok {
		return fmt.Errorf("maker exchange session %s is not defined", s.MakerExchange)
	}

	s.makerSession = makerSession

	s.sourceMarket, ok = s.sourceSession.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("source session market %s is not defined", s.Symbol)
	}

	s.makerMarket, ok = s.makerSession.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("maker session market %s is not defined", s.Symbol)
	}

	// restore state
	instanceID := fmt.Sprintf("%s-%s", ID, s.Symbol)
	s.groupID = max.GenerateGroupID(instanceID)
	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	var state State

	// load position
	if err := s.Persistence.Load(&state, ID, s.Symbol, stateKey); err != nil {
		if err != service.ErrPersistenceNotExists {
			return err
		}

		s.state = &State{}
	} else {
		// loaded successfully
		s.state = &state

		log.Infof("state is restored: %+v", s.state)
		s.Notify("%s position is restored => %f", s.Symbol, s.state.HedgePosition.Float64())
	}

	// if position is nil, we need to allocate a new position for calculation
	if s.state.Position == nil {
		s.state.Position = &bbgo.Position{
			Symbol:        s.Symbol,
			BaseCurrency:  s.makerMarket.BaseCurrency,
			QuoteCurrency: s.makerMarket.QuoteCurrency,
		}
	}

	s.book = types.NewStreamBook(s.Symbol)
	s.book.BindStream(s.sourceSession.Stream)

	s.sourceSession.Stream.OnTradeUpdate(s.handleTradeUpdate)
	s.makerSession.Stream.OnTradeUpdate(s.handleTradeUpdate)

	s.activeMakerOrders = bbgo.NewLocalActiveOrderBook()
	s.activeMakerOrders.BindStream(s.makerSession.Stream)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(s.sourceSession.Stream)
	s.orderStore.BindStream(s.makerSession.Stream)

	s.stopC = make(chan struct{})

	go func() {
		posTicker := time.NewTicker(durationJitter(s.HedgeInterval.Duration(), 200))
		defer posTicker.Stop()

		quoteTicker := time.NewTicker(durationJitter(s.UpdateInterval.Duration(), 200))
		defer quoteTicker.Stop()

		defer func() {
			if err := s.makerSession.Exchange.CancelOrders(context.Background(), s.activeMakerOrders.Orders()...); err != nil {
				log.WithError(err).Errorf("can not cancel %s orders", s.Symbol)
			}
		}()

		for {
			select {

			case <-s.stopC:
				log.Warnf("%s maker goroutine stopped, due to the stop signal", s.Symbol)
				return

			case <-ctx.Done():
				log.Warnf("%s maker goroutine stopped, due to the cancelled context", s.Symbol)
				return

			case <-quoteTicker.C:
				s.updateQuote(ctx)

			case <-posTicker.C:
				position := s.state.HedgePosition.AtomicLoad()
				abspos := math.Abs(position.Float64())
				if !s.DisableHedge && abspos > s.sourceMarket.MinQuantity {
					s.Hedge(ctx, -position)
				}
			}
		}
	}()

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		close(s.stopC)

		time.Sleep(s.UpdateInterval.Duration())

		for {
			orders := s.activeMakerOrders.Orders()
			if len(orders) == 0 {
				log.Info("all orders are cancelled successfully")
				break
			}

			log.Warnf("%d orders are not cancelled yet...", len(orders))
			s.activeMakerOrders.Print()

			if err := s.makerSession.Exchange.CancelOrders(ctx, s.activeMakerOrders.Orders()...); err != nil {
				log.WithError(err).Errorf("can not cancel %s orders", s.Symbol)
			}

			log.Warnf("waiting for orders to be cancelled...")
			time.Sleep(3 * time.Second)
		}

		if err := s.Persistence.Save(s.state, ID, s.Symbol, stateKey); err != nil {
			log.WithError(err).Errorf("can not save state: %+v", s.state)
		} else {
			log.Infof("state is saved => %+v", s.state)
			s.Notify("%s position is saved: position = %f", s.Symbol, s.state.HedgePosition.Float64())
		}
	})

	return nil
}

func durationJitter(d time.Duration, jitterInMilliseconds int) time.Duration {
	n := rand.Intn(jitterInMilliseconds)
	return d + time.Duration(n)*time.Millisecond
}
