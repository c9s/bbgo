package xmaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var defaultMargin = fixedpoint.NewFromFloat(0.003)
var Two = fixedpoint.NewFromInt(2)

const priceUpdateTimeout = 30 * time.Second

const ID = "xmaker"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment

	Symbol string `json:"symbol"`

	// SourceExchange session name
	SourceExchange string `json:"sourceExchange"`

	// MakerExchange session name
	MakerExchange string `json:"makerExchange"`

	UpdateInterval      types.Duration `json:"updateInterval"`
	HedgeInterval       types.Duration `json:"hedgeInterval"`
	OrderCancelWaitTime types.Duration `json:"orderCancelWaitTime"`

	Margin        fixedpoint.Value `json:"margin"`
	BidMargin     fixedpoint.Value `json:"bidMargin"`
	AskMargin     fixedpoint.Value `json:"askMargin"`
	UseDepthPrice bool             `json:"useDepthPrice"`
	DepthQuantity fixedpoint.Value `json:"depthQuantity"`

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

	NotifyTrade bool `json:"notifyTrade"`

	// RecoverTrade tries to find the missing trades via the REStful API
	RecoverTrade bool `json:"recoverTrade"`

	RecoverTradeScanPeriod types.Duration `json:"recoverTradeScanPeriod"`

	NumLayers int `json:"numLayers"`

	// Pips is the pips of the layer prices
	Pips fixedpoint.Value `json:"pips"`

	// --------------------------------
	// private field

	makerSession, sourceSession *bbgo.ExchangeSession

	makerMarket, sourceMarket types.Market

	// boll is the BOLLINGER indicator we used for predicting the price.
	boll *indicator.BOLL

	state *State

	// persistence fields
	Position        *types.Position  `json:"position,omitempty" persistence:"position"`
	ProfitStats     *ProfitStats     `json:"profitStats,omitempty" persistence:"profit_stats"`
	CoveredPosition fixedpoint.Value `json:"coveredPosition,omitempty" persistence:"covered_position"`

	book              *types.StreamOrderBook
	activeMakerOrders *bbgo.ActiveOrderBook

	hedgeErrorLimiter         *rate.Limiter
	hedgeErrorRateReservation *rate.Reservation

	orderStore     *core.OrderStore
	tradeCollector *core.TradeCollector

	askPriceHeartBeat, bidPriceHeartBeat *types.PriceHeartBeat

	lastPrice fixedpoint.Value
	groupID   uint32

	stopC chan struct{}
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
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
	totalAmount := fixedpoint.Zero

	if len(pvs) == 0 {
		price = fixedpoint.Zero
		return price
	} else if pvs[0].Volume.Compare(requiredQuantity) >= 0 {
		return pvs[0].Price
	}

	for i := 0; i < len(pvs); i++ {
		pv := pvs[i]
		if pv.Volume.Compare(q) >= 0 {
			totalAmount = totalAmount.Add(q.Mul(pv.Price))
			break
		}

		q = q.Sub(pv.Volume)
		totalAmount = totalAmount.Add(pv.Volume.Mul(pv.Price))
	}

	price = totalAmount.Div(requiredQuantity)
	return price
}

func (s *Strategy) Initialize() error {
	s.bidPriceHeartBeat = types.NewPriceHeartBeat(priceUpdateTimeout)
	s.askPriceHeartBeat = types.NewPriceHeartBeat(priceUpdateTimeout)
	return nil
}

func (s *Strategy) updateQuote(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter) {
	if err := s.activeMakerOrders.GracefulCancel(ctx, s.makerSession.Exchange); err != nil {
		log.Warnf("there are some %s orders not canceled, skipping placing maker orders", s.Symbol)
		s.activeMakerOrders.Print()
		return
	}

	if s.activeMakerOrders.NumOfOrders() > 0 {
		return
	}

	bestBid, bestAsk, hasPrice := s.book.BestBidAndAsk()
	if !hasPrice {
		return
	}

	// use mid-price for the last price
	s.lastPrice = bestBid.Price.Add(bestAsk.Price).Div(Two)

	bookLastUpdateTime := s.book.LastUpdateTime()

	if _, err := s.bidPriceHeartBeat.Update(bestBid); err != nil {
		log.WithError(err).Errorf("quote update error, %s price not updating, order book last update: %s ago",
			s.Symbol,
			time.Since(bookLastUpdateTime))
		return
	}

	if _, err := s.askPriceHeartBeat.Update(bestAsk); err != nil {
		log.WithError(err).Errorf("quote update error, %s price not updating, order book last update: %s ago",
			s.Symbol,
			time.Since(bookLastUpdateTime))
		return
	}

	sourceBook := s.book.CopyDepth(10)
	if valid, err := sourceBook.IsValid(); !valid {
		log.WithError(err).Errorf("%s invalid copied order book, skip quoting: %v", s.Symbol, err)
		return
	}

	var disableMakerBid = false
	var disableMakerAsk = false

	// check maker's balance quota
	// we load the balances from the account while we're generating the orders,
	// the balance may have a chance to be deducted by other strategies or manual orders submitted by the user
	makerBalances := s.makerSession.GetAccount().Balances()
	makerQuota := &bbgo.QuotaTransaction{}
	if b, ok := makerBalances[s.makerMarket.BaseCurrency]; ok {
		if b.Available.Compare(s.makerMarket.MinQuantity) > 0 {
			makerQuota.BaseAsset.Add(b.Available)
		} else {
			disableMakerAsk = true
		}
	}

	if b, ok := makerBalances[s.makerMarket.QuoteCurrency]; ok {
		if b.Available.Compare(s.makerMarket.MinNotional) > 0 {
			makerQuota.QuoteAsset.Add(b.Available)
		} else {
			disableMakerBid = true
		}
	}

	hedgeBalances := s.sourceSession.GetAccount().Balances()
	hedgeQuota := &bbgo.QuotaTransaction{}
	if b, ok := hedgeBalances[s.sourceMarket.BaseCurrency]; ok {
		// to make bid orders, we need enough base asset in the foreign exchange,
		// if the base asset balance is not enough for selling
		if s.StopHedgeBaseBalance.Sign() > 0 {
			minAvailable := s.StopHedgeBaseBalance.Add(s.sourceMarket.MinQuantity)
			if b.Available.Compare(minAvailable) > 0 {
				hedgeQuota.BaseAsset.Add(b.Available.Sub(minAvailable))
			} else {
				log.Warnf("%s maker bid disabled: insufficient base balance %s", s.Symbol, b.String())
				disableMakerBid = true
			}
		} else if b.Available.Compare(s.sourceMarket.MinQuantity) > 0 {
			hedgeQuota.BaseAsset.Add(b.Available)
		} else {
			log.Warnf("%s maker bid disabled: insufficient base balance %s", s.Symbol, b.String())
			disableMakerBid = true
		}
	}

	if b, ok := hedgeBalances[s.sourceMarket.QuoteCurrency]; ok {
		// to make ask orders, we need enough quote asset in the foreign exchange,
		// if the quote asset balance is not enough for buying
		if s.StopHedgeQuoteBalance.Sign() > 0 {
			minAvailable := s.StopHedgeQuoteBalance.Add(s.sourceMarket.MinNotional)
			if b.Available.Compare(minAvailable) > 0 {
				hedgeQuota.QuoteAsset.Add(b.Available.Sub(minAvailable))
			} else {
				log.Warnf("%s maker ask disabled: insufficient quote balance %s", s.Symbol, b.String())
				disableMakerAsk = true
			}
		} else if b.Available.Compare(s.sourceMarket.MinNotional) > 0 {
			hedgeQuota.QuoteAsset.Add(b.Available)
		} else {
			log.Warnf("%s maker ask disabled: insufficient quote balance %s", s.Symbol, b.String())
			disableMakerAsk = true
		}
	}

	// if max exposure position is configured, we should not:
	// 1. place bid orders when we already bought too much
	// 2. place ask orders when we already sold too much
	if s.MaxExposurePosition.Sign() > 0 {
		pos := s.Position.GetBase()

		if pos.Compare(s.MaxExposurePosition.Neg()) > 0 {
			// stop sell if we over-sell
			disableMakerAsk = true
		} else if pos.Compare(s.MaxExposurePosition) > 0 {
			// stop buy if we over buy
			disableMakerBid = true
		}
	}

	if disableMakerAsk && disableMakerBid {
		log.Warnf("%s bid/ask maker is disabled due to insufficient balances", s.Symbol)
		return
	}

	bestBidPrice := bestBid.Price
	bestAskPrice := bestAsk.Price
	log.Infof("%s book ticker: best ask / best bid = %v / %v", s.Symbol, bestAskPrice, bestBidPrice)

	var submitOrders []types.SubmitOrder
	var accumulativeBidQuantity, accumulativeAskQuantity fixedpoint.Value
	var bidQuantity = s.Quantity
	var askQuantity = s.Quantity
	var bidMargin = s.BidMargin
	var askMargin = s.AskMargin
	var pips = s.Pips

	if s.EnableBollBandMargin {
		lastDownBand := fixedpoint.NewFromFloat(s.boll.DownBand.Last(0))
		lastUpBand := fixedpoint.NewFromFloat(s.boll.UpBand.Last(0))

		if lastUpBand.IsZero() || lastDownBand.IsZero() {
			log.Warnf("bollinger band value is zero, skipping")
			return
		}

		log.Infof("bollinger band: up/down = %f/%f", lastUpBand.Float64(), lastDownBand.Float64())

		// when bid price is lower than the down band, then it's in the downtrend
		// when ask price is higher than the up band, then it's in the uptrend
		if bestBidPrice.Compare(lastDownBand) < 0 {
			// ratio here should be greater than 1.00
			ratio := lastDownBand.Div(bestBidPrice)

			// so that the original bid margin can be multiplied by 1.x
			bollMargin := s.BollBandMargin.Mul(ratio).Mul(s.BollBandMarginFactor)

			log.Infof("%s bollband downtrend: adjusting ask margin %v + %v = %v",
				s.Symbol,
				askMargin,
				bollMargin,
				askMargin.Add(bollMargin))

			askMargin = askMargin.Add(bollMargin)
			pips = pips.Mul(ratio)
		}

		if bestAskPrice.Compare(lastUpBand) > 0 {
			// ratio here should be greater than 1.00
			ratio := bestAskPrice.Div(lastUpBand)

			// so that the original bid margin can be multiplied by 1.x
			bollMargin := s.BollBandMargin.Mul(ratio).Mul(s.BollBandMarginFactor)

			log.Infof("%s bollband uptrend adjusting bid margin %v + %v = %v",
				s.Symbol,
				bidMargin,
				bollMargin,
				bidMargin.Add(bollMargin))

			bidMargin = bidMargin.Add(bollMargin)
			pips = pips.Mul(ratio)
		}
	}

	bidPrice := bestBidPrice
	askPrice := bestAskPrice
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

			accumulativeBidQuantity = accumulativeBidQuantity.Add(bidQuantity)
			if s.UseDepthPrice {
				if s.DepthQuantity.Sign() > 0 {
					bidPrice = aggregatePrice(sourceBook.SideBook(types.SideTypeBuy), s.DepthQuantity)
				} else {
					bidPrice = aggregatePrice(sourceBook.SideBook(types.SideTypeBuy), accumulativeBidQuantity)
				}
			}

			bidPrice = bidPrice.Mul(fixedpoint.One.Sub(bidMargin))
			if i > 0 && pips.Sign() > 0 {
				bidPrice = bidPrice.Sub(pips.Mul(fixedpoint.NewFromInt(int64(i)).
					Mul(s.makerMarket.TickSize)))
			}

			if makerQuota.QuoteAsset.Lock(bidQuantity.Mul(bidPrice)) && hedgeQuota.BaseAsset.Lock(bidQuantity) {
				// if we bought, then we need to sell the base from the hedge session
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Type:        types.OrderTypeLimit,
					Side:        types.SideTypeBuy,
					Price:       bidPrice,
					Quantity:    bidQuantity,
					TimeInForce: types.TimeInForceGTC,
					GroupID:     s.groupID,
				})

				makerQuota.Commit()
				hedgeQuota.Commit()
			} else {
				makerQuota.Rollback()
				hedgeQuota.Rollback()
			}

			if s.QuantityMultiplier.Sign() > 0 {
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
			accumulativeAskQuantity = accumulativeAskQuantity.Add(askQuantity)

			if s.UseDepthPrice {
				if s.DepthQuantity.Sign() > 0 {
					askPrice = aggregatePrice(sourceBook.SideBook(types.SideTypeSell), s.DepthQuantity)
				} else {
					askPrice = aggregatePrice(sourceBook.SideBook(types.SideTypeSell), accumulativeAskQuantity)
				}
			}

			askPrice = askPrice.Mul(fixedpoint.One.Add(askMargin))
			if i > 0 && pips.Sign() > 0 {
				askPrice = askPrice.Add(pips.Mul(fixedpoint.NewFromInt(int64(i)).Mul(s.makerMarket.TickSize)))
			}

			if makerQuota.BaseAsset.Lock(askQuantity) && hedgeQuota.QuoteAsset.Lock(askQuantity.Mul(askPrice)) {
				// if we bought, then we need to sell the base from the hedge session
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Market:      s.makerMarket,
					Type:        types.OrderTypeLimit,
					Side:        types.SideTypeSell,
					Price:       askPrice,
					Quantity:    askQuantity,
					TimeInForce: types.TimeInForceGTC,
					GroupID:     s.groupID,
				})
				makerQuota.Commit()
				hedgeQuota.Commit()
			} else {
				makerQuota.Rollback()
				hedgeQuota.Rollback()
			}

			if s.QuantityMultiplier.Sign() > 0 {
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

var lastPriceModifier = fixedpoint.NewFromFloat(1.001)
var minGap = fixedpoint.NewFromFloat(1.02)

func (s *Strategy) Hedge(ctx context.Context, pos fixedpoint.Value) {
	side := types.SideTypeBuy
	if pos.IsZero() {
		return
	}

	quantity := pos.Abs()

	if pos.Sign() < 0 {
		side = types.SideTypeSell
	}

	lastPrice := s.lastPrice
	sourceBook := s.book.CopyDepth(1)
	switch side {

	case types.SideTypeBuy:
		if bestAsk, ok := sourceBook.BestAsk(); ok {
			lastPrice = bestAsk.Price
		}

	case types.SideTypeSell:
		if bestBid, ok := sourceBook.BestBid(); ok {
			lastPrice = bestBid.Price
		}
	}

	notional := quantity.Mul(lastPrice)
	if notional.Compare(s.sourceMarket.MinNotional) <= 0 {
		log.Warnf("%s %v less than min notional, skipping hedge", s.Symbol, notional)
		return
	}

	// adjust quantity according to the balances
	account := s.sourceSession.GetAccount()
	switch side {

	case types.SideTypeBuy:
		// check quote quantity
		if quote, ok := account.Balance(s.sourceMarket.QuoteCurrency); ok {
			if quote.Available.Compare(notional) < 0 {
				// adjust price to higher 0.1%, so that we can ensure that the order can be executed
				quantity = bbgo.AdjustQuantityByMaxAmount(quantity, lastPrice.Mul(lastPriceModifier), quote.Available)
				quantity = s.sourceMarket.TruncateQuantity(quantity)
			}
		}

	case types.SideTypeSell:
		// check quote quantity
		if base, ok := account.Balance(s.sourceMarket.BaseCurrency); ok {
			if base.Available.Compare(quantity) < 0 {
				quantity = base.Available
			}
		}
	}

	// truncate quantity for the supported precision
	quantity = s.sourceMarket.TruncateQuantity(quantity)

	if notional.Compare(s.sourceMarket.MinNotional.Mul(minGap)) <= 0 {
		log.Warnf("the adjusted amount %v is less than minimal notional %v, skipping hedge", notional, s.sourceMarket.MinNotional)
		return
	}

	if quantity.Compare(s.sourceMarket.MinQuantity.Mul(minGap)) <= 0 {
		log.Warnf("the adjusted quantity %v is less than minimal quantity %v, skipping hedge", quantity, s.sourceMarket.MinQuantity)
		return
	}

	if s.hedgeErrorRateReservation != nil {
		if !s.hedgeErrorRateReservation.OK() {
			return
		}
		bbgo.Notify("Hit hedge error rate limit, waiting...")
		time.Sleep(s.hedgeErrorRateReservation.Delay())
		s.hedgeErrorRateReservation = nil
	}

	log.Infof("submitting %s hedge order %s %v", s.Symbol, side.String(), quantity)
	bbgo.Notify("Submitting %s hedge order %s %v", s.Symbol, side.String(), quantity)
	orderExecutor := &bbgo.ExchangeOrderExecutor{Session: s.sourceSession}
	returnOrders, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Market:   s.sourceMarket,
		Symbol:   s.Symbol,
		Type:     types.OrderTypeMarket,
		Side:     side,
		Quantity: quantity,
	})

	if err != nil {
		s.hedgeErrorRateReservation = s.hedgeErrorLimiter.Reserve()
		log.WithError(err).Errorf("market order submit error: %s", err.Error())
		return
	}

	// if it's selling, than we should add positive position
	if side == types.SideTypeSell {
		s.CoveredPosition = s.CoveredPosition.Add(quantity)
	} else {
		s.CoveredPosition = s.CoveredPosition.Add(quantity.Neg())
	}

	s.orderStore.Add(returnOrders...)
}

func (s *Strategy) tradeRecover(ctx context.Context) {
	tradeScanInterval := s.RecoverTradeScanPeriod.Duration()
	if tradeScanInterval == 0 {
		tradeScanInterval = 30 * time.Minute
	}

	tradeScanOverlapBufferPeriod := 5 * time.Minute

	tradeScanTicker := time.NewTicker(tradeScanInterval)
	defer tradeScanTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-tradeScanTicker.C:
			log.Infof("scanning trades from %s ago...", tradeScanInterval)

			if s.RecoverTrade {
				startTime := time.Now().Add(-tradeScanInterval).Add(-tradeScanOverlapBufferPeriod)

				if err := s.tradeCollector.Recover(ctx, s.sourceSession.Exchange.(types.ExchangeTradeHistoryService), s.Symbol, startTime); err != nil {
					log.WithError(err).Errorf("query trades error")
				}

				if err := s.tradeCollector.Recover(ctx, s.makerSession.Exchange.(types.ExchangeTradeHistoryService), s.Symbol, startTime); err != nil {
					log.WithError(err).Errorf("query trades error")
				}
			}
		}
	}
}

func (s *Strategy) Validate() error {
	if s.Quantity.IsZero() || s.QuantityScale == nil {
		return errors.New("quantity or quantityScale can not be empty")
	}

	if !s.QuantityMultiplier.IsZero() && s.QuantityMultiplier.Sign() < 0 {
		return errors.New("quantityMultiplier can not be a negative number")
	}

	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

func (s *Strategy) CrossRun(
	ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession,
) error {
	if s.BollBandInterval == "" {
		s.BollBandInterval = types.Interval1m
	}

	if s.BollBandMarginFactor.IsZero() {
		s.BollBandMarginFactor = fixedpoint.One
	}
	if s.BollBandMargin.IsZero() {
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

	if s.BidMargin.IsZero() {
		if !s.Margin.IsZero() {
			s.BidMargin = s.Margin
		} else {
			s.BidMargin = defaultMargin
		}
	}

	if s.AskMargin.IsZero() {
		if !s.Margin.IsZero() {
			s.AskMargin = s.Margin
		} else {
			s.AskMargin = defaultMargin
		}
	}

	s.hedgeErrorLimiter = rate.NewLimiter(rate.Every(1*time.Minute), 1)

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

	standardIndicatorSet := s.sourceSession.StandardIndicatorSet(s.Symbol)
	if !ok {
		return fmt.Errorf("%s standard indicator set not found", s.Symbol)
	}

	s.boll = standardIndicatorSet.BOLL(types.IntervalWindow{
		Interval: s.BollBandInterval,
		Window:   21,
	}, 1.0)

	if store, ok := s.sourceSession.MarketDataStore(s.Symbol); ok {
		if klines, ok2 := store.KLinesOfInterval(s.BollBandInterval); ok2 {
			for i := 0; i < len(*klines); i++ {
				s.boll.CalculateAndUpdate((*klines)[0 : i+1])
			}
		}
	}

	// restore state
	instanceID := s.InstanceID()
	s.groupID = util.FNV32(instanceID)
	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.makerMarket)

		// force update for legacy code
		s.Position.Market = s.makerMarket
	}

	bbgo.Notify("xmaker: %s position is restored", s.Symbol, s.Position)

	if s.ProfitStats == nil {
		s.ProfitStats = &ProfitStats{
			ProfitStats:   types.NewProfitStats(s.makerMarket),
			MakerExchange: s.makerSession.ExchangeName,
		}
	}

	if s.CoveredPosition.IsZero() {
		if s.state != nil && !s.CoveredPosition.IsZero() {
			s.CoveredPosition = s.state.CoveredPosition
		}
	}

	if s.makerSession.MakerFeeRate.Sign() > 0 || s.makerSession.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(types.ExchangeName(s.MakerExchange), types.ExchangeFee{
			MakerFeeRate: s.makerSession.MakerFeeRate,
			TakerFeeRate: s.makerSession.TakerFeeRate,
		})
	}

	if s.sourceSession.MakerFeeRate.Sign() > 0 || s.sourceSession.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(types.ExchangeName(s.SourceExchange), types.ExchangeFee{
			MakerFeeRate: s.sourceSession.MakerFeeRate,
			TakerFeeRate: s.sourceSession.TakerFeeRate,
		})
	}

	s.book = types.NewStreamBook(s.Symbol)
	s.book.BindStream(s.sourceSession.MarketDataStream)

	s.activeMakerOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(s.makerSession.UserDataStream)

	s.orderStore = core.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(s.sourceSession.UserDataStream)
	s.orderStore.BindStream(s.makerSession.UserDataStream)

	s.tradeCollector = core.NewTradeCollector(s.Symbol, s.Position, s.orderStore)

	if s.NotifyTrade {
		s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
			bbgo.Notify(trade)
		})
	}

	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		c := trade.PositionChange()
		if trade.Exchange == s.sourceSession.ExchangeName {
			s.CoveredPosition = s.CoveredPosition.Add(c)
		}

		s.ProfitStats.AddTrade(trade)

		if profit.Compare(fixedpoint.Zero) == 0 {
			s.Environment.RecordPosition(s.Position, trade, nil)
		} else {
			log.Infof("%s generated profit: %v", s.Symbol, profit)

			p := s.Position.NewProfit(trade, profit, netProfit)
			p.Strategy = ID
			p.StrategyInstanceID = instanceID
			bbgo.Notify(&p)
			s.ProfitStats.AddProfit(p)

			s.Environment.RecordPosition(s.Position, trade, &p)
		}
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		bbgo.Notify(position)
	})
	s.tradeCollector.OnRecover(func(trade types.Trade) {
		bbgo.Notify("Recovered trade", trade)
	})
	s.tradeCollector.BindStream(s.sourceSession.UserDataStream)
	s.tradeCollector.BindStream(s.makerSession.UserDataStream)

	s.stopC = make(chan struct{})

	if s.RecoverTrade {
		go s.tradeRecover(ctx)
	}

	go func() {
		posTicker := time.NewTicker(util.MillisecondsJitter(s.HedgeInterval.Duration(), 200))
		defer posTicker.Stop()

		quoteTicker := time.NewTicker(util.MillisecondsJitter(s.UpdateInterval.Duration(), 200))
		defer quoteTicker.Stop()

		reportTicker := time.NewTicker(time.Hour)
		defer reportTicker.Stop()

		defer func() {
			if err := s.activeMakerOrders.GracefulCancel(context.Background(), s.makerSession.Exchange); err != nil {
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

			case <-reportTicker.C:
				bbgo.Notify(s.ProfitStats)

			case <-posTicker.C:
				// For positive position and positive covered position:
				// uncover position = +5 - +3 (covered position) = 2
				//
				// For positive position and negative covered position:
				// uncover position = +5 - (-3) (covered position) = 8
				//
				// meaning we bought 5 on MAX and sent buy order with 3 on binance
				//
				// For negative position:
				// uncover position = -5 - -3 (covered position) = -2
				s.tradeCollector.Process()

				position := s.Position.GetBase()

				uncoverPosition := position.Sub(s.CoveredPosition)
				absPos := uncoverPosition.Abs()
				if !s.DisableHedge && absPos.Compare(s.sourceMarket.MinQuantity) > 0 {
					log.Infof("%s base position %v coveredPosition: %v uncoverPosition: %v",
						s.Symbol,
						position,
						s.CoveredPosition,
						uncoverPosition,
					)

					s.Hedge(ctx, uncoverPosition.Neg())
				}
			}
		}
	}()

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		close(s.stopC)

		// wait for the quoter to stop
		time.Sleep(s.UpdateInterval.Duration())

		shutdownCtx, cancelShutdown := context.WithTimeout(context.TODO(), time.Minute)
		defer cancelShutdown()

		if err := s.activeMakerOrders.GracefulCancel(shutdownCtx, s.makerSession.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel error")
		}

		bbgo.Notify("%s: %s position", ID, s.Symbol, s.Position)
	})

	return nil
}
