package xfunding

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/util/backoff"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xfunding"

// Position State Transitions:
// NoOp -> Opening
// Opening -> Ready -> Closing
// Closing -> Closed -> Opening
//go:generate stringer -type=PositionState
type PositionState int

const (
	PositionClosed PositionState = iota
	PositionOpening
	PositionReady
	PositionClosing
)

var log = logrus.WithField("strategy", ID)

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	PositionStartTime time.Time `json:"positionStartTime"`

	// PositionState is default to NoOp
	PositionState PositionState

	PendingBaseTransfer fixedpoint.Value `json:"pendingBaseTransfer"`
	TotalBaseTransfer   fixedpoint.Value `json:"totalBaseTransfer"`
	UsedQuoteInvestment fixedpoint.Value `json:"usedQuoteInvestment"`
}

func (s *State) Reset() {
	s.PositionState = PositionClosed
	s.PendingBaseTransfer = fixedpoint.Zero
	s.TotalBaseTransfer = fixedpoint.Zero
	s.UsedQuoteInvestment = fixedpoint.Zero
}

// Strategy is the xfunding fee strategy
// Right now it only supports short position in the USDT futures account.
// When opening the short position, it uses spot account to buy inventory, then transfer the inventory to the futures account as collateral assets.
type Strategy struct {
	Environment *bbgo.Environment

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string       `json:"symbol"`
	Market types.Market `json:"-"`

	// Leverage is the leverage of the futures position
	Leverage fixedpoint.Value `json:"leverage,omitempty"`

	// IncrementalQuoteQuantity is used for opening position incrementally with a small fixed quote quantity
	// for example, 100usdt per order
	IncrementalQuoteQuantity fixedpoint.Value `json:"incrementalQuoteQuantity"`

	QuoteInvestment fixedpoint.Value `json:"quoteInvestment"`

	MinHoldingPeriod types.Duration `json:"minHoldingPeriod"`

	// ShortFundingRate is the funding rate range for short positions
	// TODO: right now we don't support negative funding rate (long position) since it's rarer
	ShortFundingRate *struct {
		High fixedpoint.Value `json:"high"`
		Low  fixedpoint.Value `json:"low"`
	} `json:"shortFundingRate"`

	SupportDetection []struct {
		Interval types.Interval `json:"interval"`
		// MovingAverageType is the moving average indicator type that we want to use,
		// it could be SMA or EWMA
		MovingAverageType string `json:"movingAverageType"`

		// MovingAverageInterval is the interval of k-lines for the moving average indicator to calculate,
		// it could be "1m", "5m", "1h" and so on.  note that, the moving averages are calculated from
		// the k-line data we subscribed
		// MovingAverageInterval types.Interval `json:"movingAverageInterval"`
		//
		// // MovingAverageWindow is the number of the window size of the moving average indicator.
		// // The number of k-lines in the window. generally used window sizes are 7, 25 and 99 in the TradingView.
		// MovingAverageWindow int `json:"movingAverageWindow"`

		MovingAverageIntervalWindow types.IntervalWindow `json:"movingAverageIntervalWindow"`

		MinVolume fixedpoint.Value `json:"minVolume"`

		MinQuoteVolume fixedpoint.Value `json:"minQuoteVolume"`
	} `json:"supportDetection"`

	SpotSession    string `json:"spotSession"`
	FuturesSession string `json:"futuresSession"`
	Reset          bool   `json:"reset"`

	ProfitStats     *types.ProfitStats `persistence:"profit_stats"`
	SpotPosition    *types.Position    `persistence:"spot_position"`
	FuturesPosition *types.Position    `persistence:"futures_position"`

	State *State `persistence:"state"`

	// mu is used for locking state
	mu sync.Mutex

	spotSession, futuresSession             *bbgo.ExchangeSession
	spotOrderExecutor, futuresOrderExecutor *bbgo.GeneralOrderExecutor
	spotMarket, futuresMarket               types.Market

	// positionType is the futures position type
	// currently we only support short position for the positive funding rate
	positionType types.PositionType
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	// TODO: add safety check
	spotSession := sessions[s.SpotSession]
	futuresSession := sessions[s.FuturesSession]

	spotSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: types.Interval1m,
	})

	futuresSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: types.Interval1m,
	})
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Defaults() error {
	if s.Leverage.IsZero() {
		s.Leverage = fixedpoint.One
	}

	if s.MinHoldingPeriod == 0 {
		s.MinHoldingPeriod = types.Duration(3 * 24 * time.Hour)
	}

	s.positionType = types.PositionShort

	return nil
}

func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	if len(s.SpotSession) == 0 {
		return errors.New("spotSession name is required")
	}

	if len(s.FuturesSession) == 0 {
		return errors.New("futuresSession name is required")
	}

	if s.QuoteInvestment.IsZero() {
		return errors.New("quoteInvestment can not be zero")
	}

	return nil
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s", ID, s.Symbol)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	standardIndicatorSet := session.StandardIndicatorSet(s.Symbol)

	var ma types.Float64Indicator
	for _, detection := range s.SupportDetection {

		switch strings.ToLower(detection.MovingAverageType) {
		case "sma":
			ma = standardIndicatorSet.SMA(types.IntervalWindow{
				Interval: detection.MovingAverageIntervalWindow.Interval,
				Window:   detection.MovingAverageIntervalWindow.Window,
			})
		case "ema", "ewma":
			ma = standardIndicatorSet.EWMA(types.IntervalWindow{
				Interval: detection.MovingAverageIntervalWindow.Interval,
				Window:   detection.MovingAverageIntervalWindow.Window,
			})
		default:
			ma = standardIndicatorSet.EWMA(types.IntervalWindow{
				Interval: detection.MovingAverageIntervalWindow.Interval,
				Window:   detection.MovingAverageIntervalWindow.Window,
			})
		}
	}
	_ = ma

	return nil
}

func (s *Strategy) CrossRun(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()

	s.spotSession = sessions[s.SpotSession]
	s.futuresSession = sessions[s.FuturesSession]

	s.spotMarket, _ = s.spotSession.Market(s.Symbol)
	s.futuresMarket, _ = s.futuresSession.Market(s.Symbol)

	// adjust QuoteInvestment
	if b, ok := s.spotSession.Account.Balance(s.spotMarket.QuoteCurrency); ok {
		originalQuoteInvestment := s.QuoteInvestment

		// adjust available quote with the fee rate
		available := b.Available.Mul(fixedpoint.NewFromFloat(1.0 - (0.01 * 0.075)))
		s.QuoteInvestment = fixedpoint.Min(available, s.QuoteInvestment)

		if originalQuoteInvestment.Compare(s.QuoteInvestment) != 0 {
			log.Infof("adjusted quoteInvestment from %s to %s according to the balance",
				originalQuoteInvestment.String(),
				s.QuoteInvestment.String(),
			)
		}
	}

	if s.ProfitStats == nil || s.Reset {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.FuturesPosition == nil || s.Reset {
		s.FuturesPosition = types.NewPositionFromMarket(s.futuresMarket)
	}

	if s.SpotPosition == nil || s.Reset {
		s.SpotPosition = types.NewPositionFromMarket(s.spotMarket)
	}

	if s.State == nil || s.Reset {
		s.State = &State{
			PositionState:       PositionClosed,
			PendingBaseTransfer: fixedpoint.Zero,
			TotalBaseTransfer:   fixedpoint.Zero,
			UsedQuoteInvestment: fixedpoint.Zero,
		}
	}

	log.Infof("loaded spot position: %s", s.SpotPosition.String())
	log.Infof("loaded futures position: %s", s.FuturesPosition.String())

	binanceFutures := s.futuresSession.Exchange.(*binance.Exchange)
	binanceSpot := s.spotSession.Exchange.(*binance.Exchange)
	_ = binanceSpot

	s.spotOrderExecutor = s.allocateOrderExecutor(ctx, s.spotSession, instanceID, s.SpotPosition)
	s.spotOrderExecutor.TradeCollector().OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		// we act differently on the spot account
		// when opening a position, we place orders on the spot account first, then the futures account,
		// and we need to accumulate the used quote amount
		//
		// when closing a position, we place orders on the futures account first, then the spot account
		// we need to close the position according to its base quantity instead of quote quantity
		if s.positionType != types.PositionShort {
			return
		}

		switch s.State.PositionState {
		case PositionOpening:
			if trade.Side != types.SideTypeBuy {
				log.Errorf("unexpected trade side: %+v, expecting BUY trade", trade)
				return
			}

			s.mu.Lock()
			s.State.UsedQuoteInvestment = s.State.UsedQuoteInvestment.Add(trade.QuoteQuantity)
			s.mu.Unlock()

			// if we have trade, try to query the balance and transfer the balance to the futures wallet account
			// TODO: handle missing trades here. If the process crashed during the transfer, how to recover?
			if err := backoff.RetryGeneral(ctx, func() error {
				return s.transferIn(ctx, binanceSpot, s.spotMarket.BaseCurrency, trade)
			}); err != nil {
				log.WithError(err).Errorf("spot-to-futures transfer in retry failed")
				return
			}

		case PositionClosing:
			if trade.Side != types.SideTypeSell {
				log.Errorf("unexpected trade side: %+v, expecting SELL trade", trade)
				return
			}

		}
	})

	s.futuresOrderExecutor = s.allocateOrderExecutor(ctx, s.futuresSession, instanceID, s.FuturesPosition)
	s.futuresOrderExecutor.TradeCollector().OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		if s.positionType != types.PositionShort {
			return
		}

		switch s.getPositionState() {
		case PositionClosing:
			if err := backoff.RetryGeneral(ctx, func() error {
				return s.transferOut(ctx, binanceSpot, s.spotMarket.BaseCurrency, trade)
			}); err != nil {
				log.WithError(err).Errorf("spot-to-futures transfer in retry failed")
				return
			}

		}
	})

	s.futuresSession.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(kline types.KLine) {
		// s.queryAndDetectPremiumIndex(ctx, binanceFutures)
	}))

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				s.queryAndDetectPremiumIndex(ctx, binanceFutures)
				s.sync(ctx)
			}
		}
	}()

	// TODO: use go routine and time.Ticker to trigger spot sync and futures sync
	/*
		s.spotSession.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(k types.KLine) {
		}))
	*/

	return nil
}

func (s *Strategy) queryAndDetectPremiumIndex(ctx context.Context, binanceFutures *binance.Exchange) {
	premiumIndex, err := binanceFutures.QueryPremiumIndex(ctx, s.Symbol)
	if err != nil {
		log.WithError(err).Error("premium index query error")
		return
	}

	log.Info(premiumIndex)

	if changed := s.detectPremiumIndex(premiumIndex); changed {
		log.Infof("position state changed to -> %s %s", s.positionType, s.State.PositionState.String())
	}
}

func (s *Strategy) sync(ctx context.Context) {
	switch s.getPositionState() {
	case PositionOpening:
		s.increaseSpotPosition(ctx)
		s.syncFuturesPosition(ctx)
	case PositionClosing:
		s.reduceFuturesPosition(ctx)
		s.syncSpotPosition(ctx)
	}
}

func (s *Strategy) reduceFuturesPosition(ctx context.Context) {
	if s.notPositionState(PositionClosing) {
		return
	}

	futuresBase := s.FuturesPosition.GetBase() // should be negative base quantity here

	if futuresBase.Sign() > 0 {
		// unexpected error
		log.Errorf("unexpected futures position (got positive, expecting negative)")
		return
	}

	_ = s.futuresOrderExecutor.GracefulCancel(ctx)

	ticker, err := s.futuresSession.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		log.WithError(err).Errorf("can not query ticker")
		return
	}

	if futuresBase.Compare(fixedpoint.Zero) < 0 {
		orderPrice := ticker.Buy
		orderQuantity := futuresBase.Abs()
		orderQuantity = fixedpoint.Max(orderQuantity, s.futuresMarket.MinQuantity)
		orderQuantity = s.futuresMarket.AdjustQuantityByMinNotional(orderQuantity, orderPrice)
		if s.futuresMarket.IsDustQuantity(orderQuantity, orderPrice) {
			log.Infof("skip futures order with dust quantity %s, market = %+v", orderQuantity.String(), s.futuresMarket)
			return
		}

		createdOrders, err := s.futuresOrderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:     s.Symbol,
			Side:       types.SideTypeBuy,
			Type:       types.OrderTypeLimitMaker,
			Quantity:   orderQuantity,
			Price:      orderPrice,
			Market:     s.futuresMarket,
			ReduceOnly: true,
		})

		if err != nil {
			log.WithError(err).Errorf("can not submit order")
			return
		}

		log.Infof("created orders: %+v", createdOrders)
	}
}

// syncFuturesPosition syncs the futures position with the given spot position
// when the spot is transferred successfully, sync futures position
// compare spot position and futures position, increase the position size until they are the same size
func (s *Strategy) syncFuturesPosition(ctx context.Context) {
	if s.positionType != types.PositionShort {
		return
	}

	if s.notPositionState(PositionOpening) {
		return
	}

	spotBase := s.SpotPosition.GetBase()       // should be positive base quantity here
	futuresBase := s.FuturesPosition.GetBase() // should be negative base quantity here

	if spotBase.IsZero() || spotBase.Sign() < 0 {
		// skip when spot base is zero
		return
	}

	log.Infof("position comparision: %s (spot) <=> %s (futures)", spotBase.String(), futuresBase.String())

	if futuresBase.Sign() > 0 {
		// unexpected error
		log.Errorf("unexpected futures position (got positive, expecting negative)")
		return
	}

	_ = s.futuresOrderExecutor.GracefulCancel(ctx)

	ticker, err := s.futuresSession.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		log.WithError(err).Errorf("can not query ticker")
		return
	}

	// compare with the spot position and increase the position
	quoteValue, err := bbgo.CalculateQuoteQuantity(ctx, s.futuresSession, s.futuresMarket.QuoteCurrency, s.Leverage)
	if err != nil {
		log.WithError(err).Errorf("can not calculate futures account quote value")
		return
	}
	log.Infof("calculated futures account quote value = %s", quoteValue.String())

	// max futures base position (without negative sign)
	maxFuturesBasePosition := fixedpoint.Min(
		spotBase.Mul(s.Leverage),
		s.State.TotalBaseTransfer.Mul(s.Leverage))

	// if - futures position < max futures position, increase it
	if futuresBase.Neg().Compare(maxFuturesBasePosition) < 0 {
		orderPrice := ticker.Sell
		diffQuantity := maxFuturesBasePosition.Sub(futuresBase.Neg())

		if diffQuantity.Sign() < 0 {
			log.Errorf("unexpected negative position diff: %s", diffQuantity.String())
			return
		}

		log.Infof("position diff quantity: %s", diffQuantity.String())

		orderQuantity := fixedpoint.Max(diffQuantity, s.futuresMarket.MinQuantity)
		orderQuantity = s.futuresMarket.AdjustQuantityByMinNotional(orderQuantity, orderPrice)
		if s.futuresMarket.IsDustQuantity(orderQuantity, orderPrice) {
			log.Infof("skip futures order with dust quantity %s, market = %+v", orderQuantity.String(), s.futuresMarket)
			return
		}

		createdOrders, err := s.futuresOrderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimitMaker,
			Quantity: orderQuantity,
			Price:    orderPrice,
			Market:   s.futuresMarket,
		})

		if err != nil {
			log.WithError(err).Errorf("can not submit order")
			return
		}

		log.Infof("created orders: %+v", createdOrders)
	}
}

func (s *Strategy) syncSpotPosition(ctx context.Context) {
	if s.positionType != types.PositionShort {
		return
	}

	if s.notPositionState(PositionClosing) {
		return
	}

	spotBase := s.SpotPosition.GetBase()       // should be positive base quantity here
	futuresBase := s.FuturesPosition.GetBase() // should be negative base quantity here

	if spotBase.IsZero() {
		s.setPositionState(PositionClosed)
		return
	}

	// skip short spot position
	if spotBase.Sign() < 0 {
		return
	}

	log.Infof("spot/futures positions: %s (spot) <=> %s (futures)", spotBase.String(), futuresBase.String())

	if futuresBase.Sign() > 0 {
		// unexpected error
		log.Errorf("unexpected futures position (got positive, expecting negative)")
		return
	}

	_ = s.futuresOrderExecutor.GracefulCancel(ctx)

	ticker, err := s.spotSession.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		log.WithError(err).Errorf("can not query ticker")
		return
	}

	if s.SpotPosition.IsDust(ticker.Sell) {
		dust := s.SpotPosition.GetBase().Abs()
		cost := s.SpotPosition.AverageCost

		log.Warnf("spot dust loss: %f %s (average cost = %f)", dust.Float64(), s.spotMarket.BaseCurrency, cost.Float64())

		s.SpotPosition.Reset()

		s.setPositionState(PositionClosed)
		return
	}

	// spot pos size > futures pos size ==> reduce spot position
	if spotBase.Compare(futuresBase.Neg()) > 0 {
		diffQuantity := spotBase.Sub(futuresBase.Neg())

		if diffQuantity.Sign() < 0 {
			log.Errorf("unexpected negative position diff: %s", diffQuantity.String())
			return
		}

		orderPrice := ticker.Sell
		orderQuantity := diffQuantity
		if b, ok := s.spotSession.Account.Balance(s.spotMarket.BaseCurrency); ok {
			orderQuantity = fixedpoint.Min(b.Available, orderQuantity)
		}

		// avoid increase the order size
		if s.spotMarket.IsDustQuantity(orderQuantity, orderPrice) {
			log.Infof("skip futures order with dust quantity %s, market = %+v", orderQuantity.String(), s.spotMarket)
			return
		}

		createdOrders, err := s.spotOrderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimitMaker,
			Quantity: orderQuantity,
			Price:    orderPrice,
			Market:   s.futuresMarket,
		})

		if err != nil {
			log.WithError(err).Errorf("can not submit spot order")
			return
		}

		log.Infof("created spot orders: %+v", createdOrders)
	}
}

func (s *Strategy) increaseSpotPosition(ctx context.Context) {
	if s.positionType != types.PositionShort {
		log.Errorf("funding long position type is not supported")
		return
	}

	if s.notPositionState(PositionOpening) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State.UsedQuoteInvestment.Compare(s.QuoteInvestment) >= 0 {
		// stop increase the position
		s.setPositionState(PositionReady)

		// DEBUG CODE - triggering closing position automatically
		// s.startClosingPosition()
		return
	}

	_ = s.spotOrderExecutor.GracefulCancel(ctx)

	ticker, err := s.spotSession.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		log.WithError(err).Errorf("can not query ticker")
		return
	}

	leftQuota := s.QuoteInvestment.Sub(s.State.UsedQuoteInvestment)

	orderPrice := ticker.Buy
	orderQuantity := fixedpoint.Min(s.IncrementalQuoteQuantity, leftQuota).Div(orderPrice)

	log.Infof("initial spot order quantity %s", orderQuantity.String())

	orderQuantity = fixedpoint.Max(orderQuantity, s.spotMarket.MinQuantity)
	orderQuantity = s.spotMarket.AdjustQuantityByMinNotional(orderQuantity, orderPrice)

	if s.spotMarket.IsDustQuantity(orderQuantity, orderPrice) {
		return
	}

	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimitMaker,
		Quantity: orderQuantity,
		Price:    orderPrice,
		Market:   s.spotMarket,
	}

	log.Infof("placing spot order: %+v", submitOrder)

	createdOrders, err := s.spotOrderExecutor.SubmitOrders(ctx, submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not submit order")
		return
	}

	log.Infof("created orders: %+v", createdOrders)
}

func (s *Strategy) detectPremiumIndex(premiumIndex *types.PremiumIndex) bool {
	fundingRate := premiumIndex.LastFundingRate

	log.Infof("last %s funding rate: %s", s.Symbol, fundingRate.Percentage())

	if s.ShortFundingRate == nil {
		return false
	}

	switch s.getPositionState() {

	case PositionClosed:
		if fundingRate.Compare(s.ShortFundingRate.High) >= 0 {
			log.Infof("funding rate %s is higher than the High threshold %s, start opening position...",
				fundingRate.Percentage(), s.ShortFundingRate.High.Percentage())

			s.startOpeningPosition(types.PositionShort, premiumIndex.Time)
			return true
		}

	case PositionReady:
		if fundingRate.Compare(s.ShortFundingRate.Low) <= 0 {
			log.Infof("funding rate %s is lower than the Low threshold %s, start closing position...",
				fundingRate.Percentage(), s.ShortFundingRate.Low.Percentage())

			holdingPeriod := premiumIndex.Time.Sub(s.State.PositionStartTime)
			if holdingPeriod < time.Duration(s.MinHoldingPeriod) {
				log.Warnf("position holding period %s is less than %s, skip closing", holdingPeriod, s.MinHoldingPeriod.Duration())
				return false
			}

			s.startClosingPosition()
			return true
		}
	}

	return false
}

func (s *Strategy) startOpeningPosition(pt types.PositionType, t time.Time) {
	// only open a new position when there is no position
	if s.notPositionState(PositionClosed) {
		return
	}

	log.Infof("startOpeningPosition")
	s.setPositionState(PositionOpening)

	s.positionType = pt

	// reset the transfer stats
	s.State.PositionStartTime = t
	s.State.PendingBaseTransfer = fixedpoint.Zero
	s.State.TotalBaseTransfer = fixedpoint.Zero
}

func (s *Strategy) startClosingPosition() {
	// we can't close a position that is not ready
	if s.notPositionState(PositionReady) {
		return
	}

	log.Infof("startClosingPosition")
	s.setPositionState(PositionClosing)

	// reset the transfer stats
	s.State.PendingBaseTransfer = fixedpoint.Zero
}

func (s *Strategy) setPositionState(state PositionState) {
	s.mu.Lock()
	origState := s.State.PositionState
	s.State.PositionState = state
	s.mu.Unlock()
	log.Infof("position state transition: %s -> %s", origState.String(), state.String())
}

func (s *Strategy) isPositionState(state PositionState) bool {
	s.mu.Lock()
	ret := s.State.PositionState == state
	s.mu.Unlock()
	return ret
}

func (s *Strategy) getPositionState() PositionState {
	return s.State.PositionState
}

func (s *Strategy) notPositionState(state PositionState) bool {
	s.mu.Lock()
	ret := s.State.PositionState != state
	s.mu.Unlock()
	return ret
}

func (s *Strategy) allocateOrderExecutor(ctx context.Context, session *bbgo.ExchangeSession, instanceID string, position *types.Position) *bbgo.GeneralOrderExecutor {
	orderExecutor := bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, position)
	orderExecutor.SetMaxRetries(0)
	orderExecutor.BindEnvironment(s.Environment)
	orderExecutor.Bind()
	orderExecutor.TradeCollector().OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		s.ProfitStats.AddTrade(trade)
	})
	orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	return orderExecutor
}
