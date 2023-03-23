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

//go:generate stringer -type=PositionAction
type PositionAction int

const (
	PositionNoOp PositionAction = iota
	PositionOpening
	PositionClosing
)

var log = logrus.WithField("strategy", ID)

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
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

	ProfitStats     *types.ProfitStats `persistence:"profit_stats"`
	SpotPosition    *types.Position    `persistence:"spot_position"`
	FuturesPosition *types.Position    `persistence:"futures_position"`

	State *State `persistence:"state"`

	// mu is used for locking state
	mu sync.Mutex

	spotSession, futuresSession             *bbgo.ExchangeSession
	spotOrderExecutor, futuresOrderExecutor *bbgo.GeneralOrderExecutor
	spotMarket, futuresMarket               types.Market

	// positionAction is default to NoOp
	positionAction PositionAction

	// positionType is the futures position type
	// currently we only support short position for the positive funding rate
	positionType types.PositionType
}

type State struct {
	PendingBaseTransfer fixedpoint.Value `json:"pendingBaseTransfer"`
	TotalBaseTransfer   fixedpoint.Value `json:"totalBaseTransfer"`
	UsedQuoteInvestment fixedpoint.Value `json:"usedQuoteInvestment"`
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

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	for _, detection := range s.SupportDetection {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: detection.Interval,
		})
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: detection.MovingAverageIntervalWindow.Interval,
		})
	}
}

func (s *Strategy) Defaults() error {
	s.Leverage = fixedpoint.One
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

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.FuturesPosition == nil {
		s.FuturesPosition = types.NewPositionFromMarket(s.futuresMarket)
	}

	if s.SpotPosition == nil {
		s.SpotPosition = types.NewPositionFromMarket(s.spotMarket)
	}

	if s.State == nil {
		s.State = &State{
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
		if s.positionType == types.PositionShort {
			switch s.positionAction {
			case PositionOpening:
				if trade.Side != types.SideTypeBuy {
					log.Errorf("unexpected trade side: %+v, expecting BUY trade", trade)
					return
				}

				s.mu.Lock()
				defer s.mu.Unlock()

				s.State.UsedQuoteInvestment = s.State.UsedQuoteInvestment.Add(trade.QuoteQuantity)
				if s.State.UsedQuoteInvestment.Compare(s.QuoteInvestment) >= 0 {
					s.positionAction = PositionNoOp
				}

				// 1) if we have trade, try to query the balance and transfer the balance to the futures wallet account
				// TODO: handle missing trades here. If the process crashed during the transfer, how to recover?
				if err := backoff.RetryGeneric(ctx, func() error {
					return s.transferIn(ctx, binanceSpot, trade)
				}); err != nil {
					log.WithError(err).Errorf("spot-to-futures transfer in retry failed")
					return
				}

				// 2) transferred successfully, sync futures position
				// compare spot position and futures position, increase the position size until they are the same size

			case PositionClosing:
				if trade.Side != types.SideTypeSell {
					log.Errorf("unexpected trade side: %+v, expecting SELL trade", trade)
					return
				}

			}
		}
	})

	s.futuresOrderExecutor = s.allocateOrderExecutor(ctx, s.futuresSession, instanceID, s.FuturesPosition)

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

	log.Infof("premiumIndex: %+v", premiumIndex)

	if changed := s.detectPremiumIndex(premiumIndex); changed {
		log.Infof("position action: %s %s", s.positionType, s.positionAction.String())
		s.triggerPositionAction(ctx)
	}
}

// TODO: replace type binance.Exchange with an interface
func (s *Strategy) transferIn(ctx context.Context, ex *binance.Exchange, trade types.Trade) error {
	currency := s.spotMarket.BaseCurrency

	// base asset needs BUY trades
	if trade.Side == types.SideTypeSell {
		return nil
	}

	balances, err := ex.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	b, ok := balances[currency]
	if !ok {
		return fmt.Errorf("%s balance not found", currency)
	}

	// TODO: according to the fee, we might not be able to get enough balance greater than the trade quantity, we can adjust the quantity here
	if b.Available.Compare(trade.Quantity) < 0 {
		log.Infof("adding to pending base transfer: %s %s", trade.Quantity, currency)
		s.State.PendingBaseTransfer = s.State.PendingBaseTransfer.Add(trade.Quantity)
		return nil
	}

	amount := s.State.PendingBaseTransfer.Add(trade.Quantity)

	pos := s.SpotPosition.GetBase()
	rest := pos.Sub(s.State.TotalBaseTransfer)

	if rest.Sign() < 0 {
		return nil
	}

	amount = fixedpoint.Min(rest, amount)

	log.Infof("transfering futures account asset %s %s", amount, currency)
	if err := ex.TransferFuturesAccountAsset(ctx, currency, amount, types.TransferIn); err != nil {
		return err
	}

	// reset pending transfer
	s.State.PendingBaseTransfer = fixedpoint.Zero

	// record the transfer in the total base transfer
	s.State.TotalBaseTransfer = s.State.TotalBaseTransfer.Add(amount)
	return nil
}

func (s *Strategy) triggerPositionAction(ctx context.Context) {
	switch s.positionAction {
	case PositionOpening:
		s.increaseSpotPosition(ctx)
		s.syncFuturesPosition(ctx)
	case PositionClosing:
		s.reduceFuturesPosition(ctx)
		s.syncSpotPosition(ctx)
	}
}

func (s *Strategy) reduceFuturesPosition(ctx context.Context) {}

// syncFuturesPosition syncs the futures position with the given spot position
func (s *Strategy) syncFuturesPosition(ctx context.Context) {
	if s.positionType != types.PositionShort {
		return
	}

	switch s.positionAction {
	case PositionClosing:
		return
	case PositionOpening, PositionNoOp:
	}

	_ = s.futuresOrderExecutor.GracefulCancel(ctx)

	ticker, err := s.futuresSession.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		log.WithError(err).Errorf("can not query ticker")
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
			// TimeInForce: types.TimeInForceGTC,
		})

		if err != nil {
			log.WithError(err).Errorf("can not submit order")
			return
		}

		log.Infof("created orders: %+v", createdOrders)
	}
}

func (s *Strategy) syncSpotPosition(ctx context.Context) {

}

func (s *Strategy) increaseSpotPosition(ctx context.Context) {
	if s.positionType != types.PositionShort {
		log.Errorf("funding long position type is not supported")
		return
	}
	if s.positionAction != PositionOpening {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State.UsedQuoteInvestment.Compare(s.QuoteInvestment) >= 0 {
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

func (s *Strategy) detectPremiumIndex(premiumIndex *types.PremiumIndex) (changed bool) {
	fundingRate := premiumIndex.LastFundingRate

	log.Infof("last %s funding rate: %s", s.Symbol, fundingRate.Percentage())

	if s.ShortFundingRate != nil {
		if fundingRate.Compare(s.ShortFundingRate.High) >= 0 {

			log.Infof("funding rate %s is higher than the High threshold %s, start opening position...",
				fundingRate.Percentage(), s.ShortFundingRate.High.Percentage())

			s.positionAction = PositionOpening
			s.positionType = types.PositionShort
			changed = true
		} else if fundingRate.Compare(s.ShortFundingRate.Low) <= 0 {
			s.positionAction = PositionClosing
			changed = true
		}
	}

	return changed
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
