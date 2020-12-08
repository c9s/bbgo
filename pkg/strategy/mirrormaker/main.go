package mirrormaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var defaultMargin = fixedpoint.NewFromFloat(0.01)

var defaultQuantity = fixedpoint.NewFromFloat(0.001)

var log = logrus.WithField("strategy", "mirrormaker")

func init() {
	bbgo.RegisterStrategy("mirrormaker", &Strategy{})
}

type Strategy struct {
	Symbol         string `json:"symbol"`
	SourceExchange string `json:"sourceExchange"`
	MakerExchange  string `json:"makerExchange"`

	Margin             fixedpoint.Value `json:"margin"`
	BidMargin          fixedpoint.Value `json:"bidMargin"`
	AskMargin          fixedpoint.Value `json:"askMargin"`
	Quantity           fixedpoint.Value `json:"quantity"`
	QuantityMultiplier fixedpoint.Value `json:"quantityMultiplier"`

	NumLayers int `json:"numLayers"`
	Pips      int `json:"pips"`

	makerSession  *bbgo.ExchangeSession
	sourceSession *bbgo.ExchangeSession

	sourceMarket types.Market
	makerMarket  types.Market

	book              *types.StreamOrderBook
	activeMakerOrders *bbgo.LocalActiveOrderBook

	orderStore *bbgo.OrderStore

	Position  fixedpoint.Value
	lastPrice float64

	stopC chan struct{}

	*bbgo.Graceful
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	sourceSession, ok := sessions[s.SourceExchange]
	if !ok {
		panic(fmt.Errorf("source exchange %s is not defined", s.SourceExchange))
	}

	log.Infof("subscribing %s from %s", s.Symbol, s.SourceExchange)
	sourceSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
}

func (s *Strategy) updateQuote(ctx context.Context) {
	if err := s.makerSession.Exchange.CancelOrders(ctx, s.activeMakerOrders.Orders()...); err != nil {
		log.WithError(err).Errorf("can not cancel orders")
		return
	}

	// avoid unlock issue
	time.Sleep(100 * time.Millisecond)

	sourceBook := s.book.Get()
	if len(sourceBook.Bids) == 0 || len(sourceBook.Asks) == 0 {
		return
	}

	bestBidPrice := sourceBook.Bids[0].Price
	bestAskPrice := sourceBook.Asks[0].Price
	log.Infof("best bid price %f, best ask price: %f", bestBidPrice.Float64(), bestAskPrice.Float64())

	bidQuantity := s.Quantity
	bidPrice := bestBidPrice.MulFloat64(1.0 - s.BidMargin.Float64())

	askQuantity := s.Quantity
	askPrice := bestAskPrice.MulFloat64(1.0 + s.AskMargin.Float64())

	log.Infof("quote bid price: %f ask price: %f", bidPrice.Float64(), askPrice.Float64())

	var submitOrders []types.SubmitOrder

	balances := s.makerSession.Account.Balances()
	makerQuota := &bbgo.QuotaTransaction{}
	if b, ok := balances[s.makerMarket.BaseCurrency]; ok {
		makerQuota.BaseAsset.Add(b.Available)
	}
	if b, ok := balances[s.makerMarket.QuoteCurrency]; ok {
		makerQuota.QuoteAsset.Add(b.Available)
	}

	hedgeBalances := s.sourceSession.Account.Balances()
	hedgeQuota := &bbgo.QuotaTransaction{}
	if b, ok := hedgeBalances[s.sourceMarket.BaseCurrency]; ok {
		hedgeQuota.BaseAsset.Add(b.Available)
	}
	if b, ok := hedgeBalances[s.sourceMarket.QuoteCurrency]; ok {
		hedgeQuota.QuoteAsset.Add(b.Available)
	}

	log.Infof("maker quota: %+v", makerQuota)
	log.Infof("hedge quota: %+v", hedgeQuota)

	for i := 0; i < s.NumLayers; i++ {
		// bid orders
		if makerQuota.QuoteAsset.Lock(bidQuantity.Mul(bidPrice)) && hedgeQuota.BaseAsset.Lock(bidQuantity) {
			// if we bought, then we need to sell the base from the hedge session
			submitOrders = append(submitOrders, types.SubmitOrder{
				Symbol:      s.Symbol,
				Type:        types.OrderTypeLimit,
				Side:        types.SideTypeBuy,
				Price:       bidPrice.Float64(),
				Quantity:    bidQuantity.Float64(),
				TimeInForce: "GTC",
			})

			makerQuota.Commit()
			hedgeQuota.Commit()
		} else {
			makerQuota.Rollback()
			hedgeQuota.Rollback()
		}

		// ask orders
		if makerQuota.BaseAsset.Lock(askQuantity) && hedgeQuota.QuoteAsset.Lock(askQuantity.Mul(askPrice)) {
			// if we bought, then we need to sell the base from the hedge session
			submitOrders = append(submitOrders, types.SubmitOrder{
				Symbol:      s.Symbol,
				Type:        types.OrderTypeLimit,
				Side:        types.SideTypeSell,
				Price:       askPrice.Float64(),
				Quantity:    askQuantity.Float64(),
				TimeInForce: "GTC",
			})
			makerQuota.Commit()
			hedgeQuota.Commit()
		} else {
			makerQuota.Rollback()
			hedgeQuota.Rollback()
		}

		bidPrice -= fixedpoint.NewFromFloat(s.makerMarket.TickSize * float64(s.Pips))
		askPrice += fixedpoint.NewFromFloat(s.makerMarket.TickSize * float64(s.Pips))

		askQuantity = askQuantity.Mul(s.QuantityMultiplier)
		bidQuantity = bidQuantity.Mul(s.QuantityMultiplier)
	}

	if len(submitOrders) == 0 {
		return
	}

	makerOrderExecutor := &bbgo.ExchangeOrderExecutor{Session: s.makerSession}
	makerOrders, err := makerOrderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Errorf("order submit error")
		return
	}

	s.activeMakerOrders.Add(makerOrders...)
	s.orderStore.Add(makerOrders...)
}

func (s *Strategy) handleTradeUpdate(trade types.Trade) {
	log.Infof("received trade %+v", trade)
	if s.orderStore.Exists(trade.OrderID) {
		log.Infof("identified trade %d with an existing order: %d", trade.ID, trade.OrderID)

		q := fixedpoint.NewFromFloat(trade.Quantity)
		if trade.Side == types.SideTypeSell {
			q = -q
		}

		s.Position.AtomicAdd(q)

		pos := s.Position.AtomicLoad()
		log.Warnf("position changed: %f", pos.Float64())

		s.lastPrice = trade.Price
	}
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
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

	if s.Quantity == 0 {
		s.Quantity = defaultQuantity
	}

	s.book = types.NewStreamBook(s.Symbol)
	s.book.BindStream(s.sourceSession.Stream)

	s.makerSession.Stream.OnTradeUpdate(s.handleTradeUpdate)

	s.activeMakerOrders = bbgo.NewLocalActiveOrderBook()
	s.activeMakerOrders.BindStream(s.makerSession.Stream)

	s.orderStore = bbgo.NewOrderStore()
	s.orderStore.BindStream(s.makerSession.Stream)

	s.stopC = make(chan struct{})

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {

			case <-s.stopC:
				return

			case <-ctx.Done():
				return

			case <-ticker.C:
				s.updateQuote(ctx)
			}
		}
	}()

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		close(s.stopC)

		defer wg.Done()

		if err := s.makerSession.Exchange.CancelOrders(ctx, s.activeMakerOrders.Orders()...); err != nil {
			log.WithError(err).Errorf("can not cancel orders")
		}
	})

	return nil
}
