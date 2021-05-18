package xmaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/sigchan"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

var localTimeZone *time.Location

const ID = "xarb"

const stateKey = "state-v1"

var defaultFeeRate = fixedpoint.NewFromFloat(0.001)

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})

	var err error
	localTimeZone, err = time.LoadLocation("Local")
	if err != nil {
		panic(err)
	}
}

type State struct {
	Position          *bbgo.Position   `json:"position,omitempty"`
	AccumulatedVolume fixedpoint.Value `json:"accumulatedVolume,omitempty"`
	AccumulatedPnL    fixedpoint.Value `json:"accumulatedPnL,omitempty"`
	AccumulatedProfit fixedpoint.Value `json:"accumulatedProfit,omitempty"`
	AccumulatedLoss   fixedpoint.Value `json:"accumulatedLoss,omitempty"`
	AccumulatedSince  int64            `json:"accumulatedSince,omitempty"`
}

func select2(ctx context.Context, chans []sigchan.Chan) bool {
	select {
	case <-ctx.Done():
		return false

	case <-chans[0]:
		return true

	case <-chans[1]:
		return true

	}
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence

	Symbol         string           `json:"symbol"`
	MaxQuantity    fixedpoint.Value `json:"maxQuantity"`
	MinSpreadRatio fixedpoint.Value `json:"minSpreadRatio"`
	// ExchangeSessions map[string]bool `json:"exchanges"`

	sessions      map[string]*bbgo.ExchangeSession
	books         map[string]*types.StreamOrderBook
	markets       map[string]types.Market
	generalMarket *types.Market

	state *State

	orderStore *bbgo.OrderStore

	groupID uint32

	stopC chan struct{}
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	for _, session := range sessions {
		session.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
	}
}

func aggregatePrice(pvs types.PriceVolumeSlice, requiredQuantity fixedpoint.Value) (price fixedpoint.Value) {
	q := requiredQuantity
	totalAmount := fixedpoint.Value(0)

	if len(pvs) == 0 {
		price = 0
		return price
	} else if pvs[0].Volume >= requiredQuantity {
		return pvs[0].Price
	}

	for i := 0; i < len(pvs); i++ {
		pv := pvs[i]
		if pv.Volume >= q {
			totalAmount += q.Mul(pv.Price)
			break
		}

		q -= pv.Volume
		totalAmount += pv.Volume.Mul(pv.Price)
	}

	price = totalAmount.Div(requiredQuantity)
	return price
}

func (s *Strategy) check(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter) {
	// find the best price
	var bestBidPrice, bestAskPrice fixedpoint.Value
	var bestBidVolume, bestAskVolume fixedpoint.Value
	var bestBidSession, bestAskSession string

	for sessionName, streamBook := range s.books {
		book := streamBook.Get()

		if len(book.Bids) == 0 || len(book.Asks) == 0 {
			continue
		}

		if valid, err := book.IsValid(); !valid {
			log.WithError(err).Errorf("%s invalid order book, skip: %s", s.Symbol, err.Error())
			continue
		}

		if bestBid, ok := book.BestBid(); ok {
			if bestBidPrice == 0 || bestBid.Price > bestBidPrice {
				bestBidPrice = bestBid.Price
				bestBidVolume = bestBid.Volume
				bestBidSession = sessionName
			}
		}

		if bestAsk, ok := book.BestAsk(); ok {
			if bestAskPrice == 0 || bestAsk.Price < bestAskPrice {
				bestAskPrice = bestAsk.Price
				bestAskVolume = bestAsk.Volume
				bestAskSession = sessionName
			}
		}
	}

	if bestBidPrice == 0 || bestAskPrice == 0 {
		return
	}

	// adjust price according to the fee
	if session, ok := s.sessions[bestBidSession]; ok {
		if session.TakerFeeRate > 0 {
			bestBidPrice = bestBidPrice.Mul(fixedpoint.NewFromFloat(1.0) + session.TakerFeeRate)
		} else {
			bestBidPrice = bestBidPrice.Mul(fixedpoint.NewFromFloat(1.0) + defaultFeeRate)
		}
	}

	if session, ok := s.sessions[bestAskSession]; ok {
		if session.TakerFeeRate > 0 {
			bestAskPrice = bestAskPrice.Mul(fixedpoint.NewFromFloat(1.0) - session.TakerFeeRate)
		} else {
			bestAskPrice = bestAskPrice.Mul(fixedpoint.NewFromFloat(1.0) - defaultFeeRate)
		}
	}

	// bid price is for selling, ask price is for buying
	if bestBidPrice < bestAskPrice {
		return
	}

	// if > 0.2% percent profit
	spreadRatio := bestBidPrice.Div(bestAskPrice).Float64()
	if spreadRatio < s.MinSpreadRatio.Float64() {
		log.Infof("spread ratio %f < %f min spread ratio, %f/%f, skipping", spreadRatio, s.MinSpreadRatio.Float64(), bestAskPrice.Float64(), bestBidPrice.Float64())
		return
	}

	log.Infof("💵 spread ratio %f > %f min spread ratio, %f/%f", spreadRatio, s.MinSpreadRatio.Float64(), bestAskPrice.Float64(), bestBidPrice.Float64())

	quantity := fixedpoint.Min(bestAskVolume, bestBidVolume)

	if s.MaxQuantity > 0 {
		quantity = fixedpoint.Min(s.MaxQuantity, quantity)
	}

	if buySession, ok := s.sessions[bestAskSession]; ok {
		if b, ok := buySession.Account.Balance(s.generalMarket.QuoteCurrency); ok {
			if b.Available.Float64() < s.generalMarket.MinNotional {
				log.Warnf("insufficient quote balance %f < %f", b.Available.Float64(), s.generalMarket.MinNotional)
				return
			}

			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, bestAskPrice, b.Available)
		}
	}

	if sellSession, ok := s.sessions[bestBidSession]; ok {
		if b, ok := sellSession.Account.Balance(s.generalMarket.BaseCurrency); ok {
			if b.Available.Float64() < s.generalMarket.MinQuantity {
				log.Warnf("insufficient base balance %f < %f", b.Available.Float64(), s.generalMarket.MinQuantity)
				return
			}

			quantity = fixedpoint.Min(quantity, b.Available)
		}
	}

	s.Notifiability.Notify("Submitting arbitrage orders: %s %f", s.Symbol, quantity.Float64())

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		createdOrders, err := orderExecutionRouter.SubmitOrdersTo(ctx, bestAskSession, types.SubmitOrder{
			Symbol:   s.Symbol,
			Type:     types.OrderTypeMarket,
			Side:     types.SideTypeBuy,
			Quantity: quantity.Float64(),
			// Price:       askPrice.Float64(),
			// TimeInForce: "GTC",
			GroupID: s.groupID,
		})
		if err != nil {
			log.WithError(err).Errorf("order error: %s", err.Error())
			return
		}

		s.orderStore.Add(createdOrders...)
	}()

	go func() {
		defer wg.Done()

		createdOrders, err := orderExecutionRouter.SubmitOrdersTo(ctx, bestBidSession, types.SubmitOrder{
			Symbol:   s.Symbol,
			Type:     types.OrderTypeMarket,
			Side:     types.SideTypeSell,
			Quantity: quantity.Float64(),
			// Price:       askPrice.Float64(),
			// TimeInForce: "GTC",
			GroupID: s.groupID,
		})
		if err != nil {
			log.WithError(err).Errorf("order error: %s", err.Error())
			return
		}

		s.orderStore.Add(createdOrders...)
	}()

	wg.Wait()
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
		log.Warnf("ignore self trade")
		return

	default:
		log.Infof("ignore non sell/buy side trades, got: %v", trade.Side)
		return

	}

	log.Infof("identified %s trade %d with an existing order: %d", trade.Symbol, trade.ID, trade.OrderID)

	s.state.AccumulatedVolume.AtomicAdd(fixedpoint.NewFromFloat(trade.Quantity))

	if profit, madeProfit := s.state.Position.AddTrade(trade); madeProfit {
		s.state.AccumulatedPnL.AtomicAdd(profit)

		if profit < 0 {
			s.state.AccumulatedLoss.AtomicAdd(profit)
		} else if profit > 0 {
			s.state.AccumulatedProfit.AtomicAdd(profit)
		}

		profitMargin := profit.DivFloat64(trade.QuoteQuantity)

		var since time.Time
		if s.state.AccumulatedSince > 0 {
			since = time.Unix(s.state.AccumulatedSince, 0).In(localTimeZone)
		}

		s.Notify("%s arbitrage profit %s %f %s (%.3f%%), since %s accumulated net profit %f %s, accumulated loss %f %s",
			s.Symbol,
			pnlEmoji(profit),
			profit.Float64(), s.state.Position.QuoteCurrency,
			profitMargin.Float64()*100.0,
			since.Format(time.RFC822),
			s.state.AccumulatedPnL.Float64(), s.state.Position.QuoteCurrency,
			s.state.AccumulatedLoss.Float64(), s.state.Position.QuoteCurrency)

	} else {
		s.Notify(s.state.Position)
	}

}

func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

func (s *Strategy) SaveState() error {
	if err := s.Persistence.Save(s.state, ID, s.Symbol, stateKey); err != nil {
		return err
	} else {
		log.Infof("state is saved => %+v", s.state)
	}
	return nil
}

func (s *Strategy) LoadState() error {
	var state State

	// load position
	if err := s.Persistence.Load(&state, ID, s.Symbol, stateKey); err != nil {
		if err != service.ErrPersistenceNotExists {
			return err
		}

		s.state = &State{}
	} else {
		s.state = &state
		log.Infof("state is restored: %+v", s.state)
	}

	return nil
}

func (s *Strategy) CrossRun(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	if s.MinSpreadRatio == 0 {
		s.MinSpreadRatio = fixedpoint.NewFromFloat(1.03)
	}

	s.sessions = make(map[string]*bbgo.ExchangeSession)
	s.books = make(map[string]*types.StreamOrderBook)
	s.markets = make(map[string]types.Market)
	s.orderStore = bbgo.NewOrderStore(s.Symbol)

	for sessionID := range sessions {
		session := sessions[sessionID]

		s.sessions[sessionID] = session

		market, ok := session.Market(s.Symbol)
		if !ok {
			return fmt.Errorf("source session market %s is not defined", s.Symbol)
		}

		s.markets[sessionID] = market

		if s.generalMarket == nil {
			s.generalMarket = &market
		}

		book := types.NewStreamBook(s.Symbol)
		book.BindStream(session.Stream)
		s.books[sessionID] = book

		session.Stream.OnTradeUpdate(s.handleTradeUpdate)

		s.orderStore.BindStream(session.Stream)
	}

	// restore state
	instanceID := fmt.Sprintf("%s-%s", ID, s.Symbol)
	s.groupID = max.GenerateGroupID(instanceID)
	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	if err := s.LoadState(); err != nil {
		return err
	}

	// if position is nil, we need to allocate a new position for calculation
	if s.state.Position == nil {
		s.state.Position = &bbgo.Position{
			Symbol:        s.Symbol,
			BaseCurrency:  s.generalMarket.BaseCurrency,
			QuoteCurrency: s.generalMarket.QuoteCurrency,
		}
	}

	// initialize fee rates
	for _, session := range sessions {
		if session.MakerFeeRate > 0 || session.TakerFeeRate > 0 {
			s.state.Position.SetExchangeFeeRate(types.ExchangeName(session.Name), bbgo.ExchangeFee{
				MakerFeeRate: session.MakerFeeRate,
				TakerFeeRate: session.TakerFeeRate,
			})
		}
	}

	if s.state.AccumulatedSince == 0 {
		s.state.AccumulatedSince = time.Now().Unix()
	}

	s.stopC = make(chan struct{})

	go func() {
		var chans []sigchan.Chan
		for n := range s.books {
			chans = append(chans, s.books[n].C)
		}

		if len(chans) > 2 {
			log.Fatal("2+ channels are not supported")
		}

		for {

			select {
			case <-s.stopC:
				log.Warnf("%s maker goroutine stopped, due to the stop signal", s.Symbol)
				return

			case <-ctx.Done():
				log.Warnf("%s maker goroutine stopped, due to the cancelled context", s.Symbol)
				return

			default:
			}

			if select2(ctx, chans) {
				s.check(ctx, orderExecutionRouter)
				// check books
			}
		}
	}()

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		close(s.stopC)

		if err := s.SaveState(); err != nil {
			log.WithError(err).Errorf("can not save state: %+v", s.state)
		}
	})

	return nil
}

// lets move this to the fun package
var lossEmoji = "🔥"
var profitEmoji = "💰"

func pnlEmoji(pnl fixedpoint.Value) string {
	if pnl < 0 {
		return lossEmoji
	}

	if pnl == 0 {
		return ""
	}

	return profitEmoji
}
