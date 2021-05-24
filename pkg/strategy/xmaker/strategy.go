package xmaker

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

var defaultMargin = fixedpoint.NewFromFloat(0.003)

var localTimeZone *time.Location

const ID = "xmaker"

const stateKey = "state-v1"

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
	HedgePosition     fixedpoint.Value `json:"hedgePosition"`
	Position          *bbgo.Position   `json:"position,omitempty"`
	AccumulatedVolume fixedpoint.Value `json:"accumulatedVolume,omitempty"`
	AccumulatedPnL    fixedpoint.Value `json:"accumulatedPnL,omitempty"`
	AccumulatedProfit fixedpoint.Value `json:"accumulatedProfit,omitempty"`
	AccumulatedLoss   fixedpoint.Value `json:"accumulatedLoss,omitempty"`
	AccumulatedSince  int64            `json:"accumulatedSince,omitempty"`
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

	EnableBollBandMargin bool             `json:"enableBollBandMargin"`
	BollBandInterval     types.Interval   `json:"bollBandInterval"`
	BollBandMargin       fixedpoint.Value `json:"bollBandMargin"`
	BollBandMarginFactor fixedpoint.Value `json:"bollBandMarginFactor"`

	StopHedgeQuoteBalance fixedpoint.Value `json:"stopHedgeQuoteBalance"`
	StopHedgeBaseBalance  fixedpoint.Value `json:"stopHedgeBaseBalance"`

	// Quantity is used for fixed quantity of the first layer
	Quantity fixedpoint.Value `json:"quantity"`

	// QuantityMultiplier is the factor that multiplies the quantity of the previous layer
	QuantityMultiplier fixedpoint.Value `json:"quantityMultiplier"`

	// QuantityScale helps user to define the quantity by layer scale
	QuantityScale *bbgo.LayerScale `json:"quantityScale,omitempty"`

	// MaxExposurePosition defines the unhedged quantity of stop
	MaxExposurePosition fixedpoint.Value `json:"maxExposurePosition"`

	DisableHedge bool `json:"disableHedge"`

	NumLayers int `json:"numLayers"`

	// Pips is the pips of the layer prices
	Pips fixedpoint.Value `json:"pips"`

	// --------------------------------
	// private field

	makerSession  *bbgo.ExchangeSession
	sourceSession *bbgo.ExchangeSession

	sourceMarket types.Market
	makerMarket  types.Market

	// boll is the BOLLINGER indicator we used for predicting the price.
	boll *indicator.BOLL

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
	sourceSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})

	makerSession, ok := sessions[s.MakerExchange]
	if !ok {
		panic(fmt.Errorf("maker session %s is not defined", s.MakerExchange))
	}
	makerSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
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

func (s *Strategy) updateQuote(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter) {
	if err := s.makerSession.Exchange.CancelOrders(ctx, s.activeMakerOrders.Orders()...); err != nil {
		log.WithError(err).Errorf("can not cancel %s orders", s.Symbol)
		return
	}

	// avoid unlock issue and wait for the balance update
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

	sourceBook := s.book.Copy()
	if valid, err := sourceBook.IsValid(); !valid {
		log.WithError(err).Errorf("%s invalid order book, skip quoting: %v", s.Symbol, err)
		return
	}

	var disableMakerBid = false
	var disableMakerAsk = false

	// check maker's balance quota
	// we load the balances from the account while we're generating the orders,
	// the balance may have a chance to be deducted by other strategies or manual orders submitted by the user
	makerBalances := s.makerSession.Account.Balances()
	makerQuota := &bbgo.QuotaTransaction{}
	if b, ok := makerBalances[s.makerMarket.BaseCurrency]; ok {
		if b.Available.Float64() > s.makerMarket.MinQuantity {
			makerQuota.BaseAsset.Add(b.Available)
		} else {
			disableMakerAsk = true
		}
	}

	if b, ok := makerBalances[s.makerMarket.QuoteCurrency]; ok {
		if b.Available.Float64() > s.makerMarket.MinNotional {
			makerQuota.QuoteAsset.Add(b.Available)
		} else {
			disableMakerBid = true
		}
	}

	hedgeBalances := s.sourceSession.Account.Balances()
	hedgeQuota := &bbgo.QuotaTransaction{}
	if b, ok := hedgeBalances[s.sourceMarket.BaseCurrency]; ok {
		// to make bid orders, we need enough base asset in the foreign exchange,
		// if the base asset balance is not enough for selling
		if s.StopHedgeBaseBalance > 0 && b.Available > (s.StopHedgeBaseBalance+fixedpoint.NewFromFloat(s.sourceMarket.MinQuantity)) {
			hedgeQuota.BaseAsset.Add(b.Available - s.StopHedgeBaseBalance - fixedpoint.NewFromFloat(s.sourceMarket.MinQuantity))
		} else if b.Available.Float64() > s.sourceMarket.MinQuantity {
			hedgeQuota.BaseAsset.Add(b.Available)
		} else {
			log.Warnf("%s maker bid disabled: insufficient base balance %s", s.Symbol, b.String())
			disableMakerBid = true
		}
	}

	if b, ok := hedgeBalances[s.sourceMarket.QuoteCurrency]; ok {
		// to make ask orders, we need enough quote asset in the foreign exchange,
		// if the quote asset balance is not enough for buying
		if s.StopHedgeQuoteBalance > 0 && b.Available > (s.StopHedgeQuoteBalance+fixedpoint.NewFromFloat(s.sourceMarket.MinNotional)) {
			hedgeQuota.QuoteAsset.Add(b.Available - s.StopHedgeQuoteBalance - fixedpoint.NewFromFloat(s.sourceMarket.MinNotional))
		} else if b.Available.Float64() > s.sourceMarket.MinNotional {
			hedgeQuota.QuoteAsset.Add(b.Available)
		} else {
			log.Warnf("%s maker ask disabled: insufficient quote balance %s", s.Symbol, b.String())
			disableMakerAsk = true
		}
	}

	// if max exposure position is configured, we should not:
	// 1. place bid orders when we already bought too much
	// 2. place ask orders when we already sold too much
	if s.MaxExposurePosition > 0 {
		pos := s.state.HedgePosition.AtomicLoad()
		if pos < -s.MaxExposurePosition {
			// stop sell if we over-sell
			disableMakerAsk = true
		} else if pos > s.MaxExposurePosition {
			// stop buy if we over buy
			disableMakerBid = true
		}
	}

	if disableMakerAsk && disableMakerBid {
		log.Warnf("%s bid/ask maker is disabled due to insufficient balances", s.Symbol)
		return
	}

	bestBid, _ := sourceBook.BestBid()
	bestBidPrice := bestBid.Price

	bestAsk, _ := sourceBook.BestAsk()
	bestAskPrice := bestAsk.Price
	log.Infof("%s book ticker: best ask / best bid = %f / %f", s.Symbol, bestAskPrice.Float64(), bestBidPrice.Float64())

	var submitOrders []types.SubmitOrder
	var accumulativeBidQuantity, accumulativeAskQuantity fixedpoint.Value
	var bidQuantity = s.Quantity
	var askQuantity = s.Quantity
	var bidMargin = s.BidMargin
	var askMargin = s.AskMargin
	var pips = s.Pips

	if s.EnableBollBandMargin {
		lastDownBand := s.boll.LastDownBand()
		lastUpBand := s.boll.LastUpBand()

		// when bid price is lower than the down band, then it's in the downtrend
		// when ask price is higher than the up band, then it's in the uptrend
		if bestBidPrice.Float64() < lastDownBand {
			// ratio here should be greater than 1.00
			ratio := lastDownBand / bestBidPrice.Float64()

			// so that the original bid margin can be multiplied by 1.x
			bollMargin := s.BollBandMargin.MulFloat64(ratio).Mul(s.BollBandMarginFactor)

			log.Infof("%s bollband downtrend: adjusting ask margin %f + %f = %f",
				s.Symbol,
				askMargin.Float64(),
				bollMargin.Float64(),
				(askMargin + bollMargin).Float64())

			askMargin = askMargin + bollMargin
			pips = pips.MulFloat64(ratio)
		}

		if bestAskPrice.Float64() > lastUpBand {
			// ratio here should be greater than 1.00
			ratio := bestAskPrice.Float64() / lastUpBand

			// so that the original bid margin can be multiplied by 1.x
			bollMargin := s.BollBandMargin.MulFloat64(ratio).Mul(s.BollBandMarginFactor)

			log.Infof("%s bollband uptrend adjusting bid margin %f + %f = %f",
				s.Symbol,
				bidMargin.Float64(),
				bollMargin.Float64(),
				(bidMargin + bollMargin).Float64())

			bidMargin = bidMargin + bollMargin
			pips = pips.MulFloat64(ratio)
		}
	}

	for i := 0; i < s.NumLayers; i++ {
		// for maker bid orders
		if !disableMakerBid {
			if s.QuantityScale != nil {
				qf, err := s.QuantityScale.Scale(i + 1)
				if err != nil {
					log.WithError(err).Errorf("quantityScale error")
					return
				}

				log.Infof("%s scaling bid #%d quantity to %f", s.Symbol, i+1, qf)

				// override the default bid quantity
				bidQuantity = fixedpoint.NewFromFloat(qf)
			}

			accumulativeBidQuantity += bidQuantity
			bidPrice := aggregatePrice(sourceBook.SideBook(types.SideTypeBuy), accumulativeBidQuantity)
			bidPrice = bidPrice.MulFloat64(1.0 - bidMargin.Float64())

			if i > 0 && pips > 0 {
				bidPrice -= pips.MulFloat64(s.makerMarket.TickSize)
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

				log.Infof("%s scaling ask #%d quantity to %f", s.Symbol, i+1, qf)

				// override the default bid quantity
				askQuantity = fixedpoint.NewFromFloat(qf)
			}
			accumulativeAskQuantity += askQuantity

			askPrice := aggregatePrice(sourceBook.SideBook(types.SideTypeSell), accumulativeAskQuantity)
			askPrice = askPrice.MulFloat64(1.0 + askMargin.Float64())
			if i > 0 && pips > 0 {
				askPrice -= pips.MulFloat64(s.makerMarket.TickSize)
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

			if s.QuantityMultiplier > 0 {
				askQuantity = askQuantity.Mul(s.QuantityMultiplier)
			}
		}
	}

	if len(submitOrders) == 0 {
		log.Warnf("no orders generated")
		return
	}

	makerOrders, err := orderExecutionRouter.SubmitOrdersTo(ctx, s.MakerExchange, submitOrders...)
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

		// quantity must be a positive number
		quantity = -pos
	}

	lastPrice := s.lastPrice
	sourceBook := s.book.Copy()
	switch side {

	case types.SideTypeBuy:
		if bestAsk, ok := sourceBook.BestAsk(); ok {
			lastPrice = bestAsk.Price.Float64()
		}

	case types.SideTypeSell:
		if bestBid, ok := sourceBook.BestBid(); ok {
			lastPrice = bestBid.Price.Float64()
		}
	}

	notional := quantity.MulFloat64(lastPrice)
	if notional.Float64() <= s.sourceMarket.MinNotional {
		log.Warnf("%s %f less than min notional, skipping", s.Symbol, notional.Float64())
		return
	}

	// adjust quantity according to the balances
	account := s.sourceSession.Account
	switch side {

	case types.SideTypeBuy:
		// check quote quantity
		if quote, ok := account.Balance(s.sourceMarket.QuoteCurrency); ok {
			if quote.Available < notional {
				// adjust price to higher 0.1%, so that we can ensure that the order can be executed
				quantity = bbgo.AdjustQuantityByMaxAmount(quantity, fixedpoint.NewFromFloat(lastPrice*1.001), quote.Available)
			}
		}

	case types.SideTypeSell:
		// check quote quantity
		if base, ok := account.Balance(s.sourceMarket.BaseCurrency); ok {
			if base.Available < quantity {
				quantity = base.Available
			}
		}

	}

	s.Notifiability.Notify("Submitting %s hedge order %s %f", s.Symbol, side.String(), quantity.Float64())
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
		log.Warnf("ignore self trade")
		return

	default:
		log.Infof("ignore non sell/buy side trades, got: %v", trade.Side)
		return

	}

	log.Infof("identified %s trade %d with an existing order: %d", trade.Symbol, trade.ID, trade.OrderID)

	s.state.HedgePosition.AtomicAdd(q)
	s.state.AccumulatedVolume.AtomicAdd(fixedpoint.NewFromFloat(trade.Quantity))

	if profit, netProfit, madeProfit := s.state.Position.AddTrade(trade); madeProfit {
		s.state.AccumulatedPnL.AtomicAdd(profit)

		if profit < 0 {
			s.state.AccumulatedLoss.AtomicAdd(profit)
		} else if profit > 0 {
			s.state.AccumulatedProfit.AtomicAdd(profit)
		}

		profitMargin := profit.DivFloat64(trade.QuoteQuantity)
		netProfitMargin := netProfit.DivFloat64(trade.QuoteQuantity)

		var since time.Time
		if s.state.AccumulatedSince > 0 {
			since = time.Unix(s.state.AccumulatedSince, 0).In(localTimeZone)
		}

		s.Notify("%s trade profit %s %f %s (%.2f%%), net profit =~ %f %s (%.2f%%), since %s accumulated net profit %f %s, accumulated loss %f %s",
			s.Symbol,
			pnlEmoji(profit),
			profit.Float64(), s.state.Position.QuoteCurrency,
			profitMargin.Float64()*100.0,
			netProfit.Float64(), s.state.Position.QuoteCurrency,
			netProfitMargin.Float64()*100.0,
			since.Format(time.RFC822),
			s.state.AccumulatedPnL.Float64(), s.state.Position.QuoteCurrency,
			s.state.AccumulatedLoss.Float64(), s.state.Position.QuoteCurrency)

	} else {
		s.Notify(s.state.Position)
	}

	s.lastPrice = trade.Price

	if err := s.SaveState(); err != nil {
		log.WithError(err).Error("save state error")
	}
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

func (s *Strategy) SaveState() error {
	if err := s.Persistence.Save(s.state, ID, s.Symbol, stateKey); err != nil {
		return err
	} else {
		log.Infof("state is saved => %+v", s.state)
	}
	return nil
}

func (s *Strategy) CrossRun(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	if s.BollBandInterval == "" {
		s.BollBandInterval = types.Interval1m
	}

	if s.BollBandMarginFactor == 0 {
		s.BollBandMarginFactor = fixedpoint.NewFromFloat(1.0)
	}
	if s.BollBandMargin == 0 {
		s.BollBandMargin = fixedpoint.NewFromFloat(0.001)
	}

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

	standardIndicatorSet, ok := s.sourceSession.StandardIndicatorSet(s.Symbol)
	if !ok {
		return fmt.Errorf("%s standard indicator set not found", s.Symbol)
	}

	s.boll = standardIndicatorSet.BOLL(types.IntervalWindow{
		Interval: s.BollBandInterval,
		Window:   21,
	}, 1.0)

	// restore state
	instanceID := fmt.Sprintf("%s-%s", ID, s.Symbol)
	s.groupID = max.GenerateGroupID(instanceID)
	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	if err := s.LoadState(); err != nil {
		return err
	} else {
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

	if s.makerSession.MakerFeeRate > 0 || s.makerSession.TakerFeeRate > 0 {
		s.state.Position.SetExchangeFeeRate(types.ExchangeName(s.MakerExchange), bbgo.ExchangeFee{
			MakerFeeRate: s.makerSession.MakerFeeRate,
			TakerFeeRate: s.makerSession.TakerFeeRate,
		})
	}

	if s.sourceSession.MakerFeeRate > 0 || s.sourceSession.TakerFeeRate > 0 {
		s.state.Position.SetExchangeFeeRate(types.ExchangeName(s.SourceExchange), bbgo.ExchangeFee{
			MakerFeeRate: s.sourceSession.MakerFeeRate,
			TakerFeeRate: s.sourceSession.TakerFeeRate,
		})
	}

	if s.state.AccumulatedSince == 0 {
		s.state.AccumulatedSince = time.Now().Unix()
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
				s.updateQuote(ctx, orderExecutionRouter)

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

		// wait for the quoter to stop
		time.Sleep(s.UpdateInterval.Duration())

		// ensure every order is cancelled
		for s.activeMakerOrders.NumOfOrders() > 0 {
			orders := s.activeMakerOrders.Orders()
			log.Warnf("%d orders are not cancelled yet:", len(orders))
			s.activeMakerOrders.Print()

			if err := s.makerSession.Exchange.CancelOrders(ctx, s.activeMakerOrders.Orders()...); err != nil {
				log.WithError(err).Errorf("can not cancel %s orders", s.Symbol)
				continue
			}

			log.Infof("waiting for orders to be cancelled...")

			select {
			case <-time.After(3 * time.Second):

			case <-ctx.Done():
				break

			}

			// verify the current open orders via the RESTful API
			if s.activeMakerOrders.NumOfOrders() > 0 {
				log.Warnf("there are orders not cancelled, using REStful API to verify...")
				openOrders, err := s.makerSession.Exchange.QueryOpenOrders(ctx, s.Symbol)
				if err != nil {
					log.WithError(err).Errorf("can not query %s open orders", s.Symbol)
					continue
				}

				openOrderStore := bbgo.NewOrderStore(s.Symbol)
				openOrderStore.Add(openOrders...)

				for _, o := range s.activeMakerOrders.Orders() {
					// if it does not exist, we should remove it
					if !openOrderStore.Exists(o.OrderID) {
						s.activeMakerOrders.Remove(o)
					}
				}
			}
		}
		log.Info("all orders are cancelled successfully")

		if err := s.SaveState(); err != nil {
			log.WithError(err).Errorf("can not save state: %+v", s.state)
		} else {
			s.Notify("%s position is saved: position = %f", s.Symbol, s.state.HedgePosition.Float64(), s.state.Position)
		}
	})

	return nil
}

func durationJitter(d time.Duration, jitterInMilliseconds int) time.Duration {
	n := rand.Intn(jitterInMilliseconds)
	return d + time.Duration(n)*time.Millisecond
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
