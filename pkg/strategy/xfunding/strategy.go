package xfunding

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/util/backoff"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

// WIP:
// - track fee token price for cost
// - buy enough BNB before creating positions
// - transfer the rest BNB into the futures account
// - add slack notification support
// - use neutral position to calculate the position cost
// - customize profit stats for this funding fee strategy

const ID = "xfunding"

// Position State Transitions:
// NoOp -> Opening
// Opening -> Ready -> Closing
// Closing -> Closed -> Opening
//
//go:generate stringer -type=PositionState
type PositionState int

const (
	PositionClosed PositionState = iota
	PositionOpening
	PositionReady
	PositionClosing
)

type MovingAverageConfig struct {
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
}

var log = logrus.WithField("strategy", ID)

var errNotBinanceExchange = errors.New("not binance exchange, currently only support binance exchange")

var errDuplicatedFundingFeeTxnId = errors.New("duplicated funding fee txn id")

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

func newState() *State {
	return &State{
		PositionState:       PositionClosed,
		PendingBaseTransfer: fixedpoint.Zero,
		TotalBaseTransfer:   fixedpoint.Zero,
		UsedQuoteInvestment: fixedpoint.Zero,
	}
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
	Symbol   string         `json:"symbol"`
	Interval types.Interval `json:"interval"`

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

	SpotSession    string `json:"spotSession"`
	FuturesSession string `json:"futuresSession"`

	// Reset your position info
	Reset bool `json:"reset"`

	ProfitFixerConfig *common.ProfitFixerConfig `json:"profitFixer"`

	// CloseFuturesPosition can be enabled to close the futures position and then transfer the collateral asset back to the spot account.
	CloseFuturesPosition bool `json:"closeFuturesPosition"`

	ProfitStats *ProfitStats `persistence:"profit_stats"`

	// SpotPosition is used for the spot position (usually long position)
	// so that we know how much spot we have bought and the average cost of the spot.
	SpotPosition *types.Position `persistence:"spot_position"`

	// FuturesPosition is used for the futures position
	// this position is the reverse side of the spot position, when spot position is long, then the futures position will be short.
	// but the base quantity should be the same as the spot position
	FuturesPosition *types.Position `persistence:"futures_position"`

	// NeutralPosition is used for sharing spot/futures position
	// when creating the spot position and futures position, there will be a spread between the spot position and the futures position.
	// this neutral position can calculate the spread cost between these two positions
	NeutralPosition *types.Position `persistence:"neutral_position"`

	State *State `persistence:"state"`

	// mu is used for locking state
	mu sync.Mutex

	spotSession, futuresSession             *bbgo.ExchangeSession
	spotOrderExecutor, futuresOrderExecutor *bbgo.GeneralOrderExecutor
	spotMarket, futuresMarket               types.Market

	binanceFutures, binanceSpot *binance.Exchange

	// positionType is the futures position type
	// currently we only support short position for the positive funding rate
	positionType types.PositionType

	minQuantity fixedpoint.Value
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	// TODO: add safety check
	spotSession := sessions[s.SpotSession]
	futuresSession := sessions[s.FuturesSession]

	spotSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	futuresSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Defaults() error {
	if s.Leverage.IsZero() {
		s.Leverage = fixedpoint.One
	}

	if s.MinHoldingPeriod == 0 {
		s.MinHoldingPeriod = types.Duration(3 * 24 * time.Hour)
	}

	if s.Interval == "" {
		s.Interval = types.Interval1m
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
	// standardIndicatorSet := session.StandardIndicatorSet(s.Symbol)
	/*
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
	*/
	return nil
}

func (s *Strategy) CrossRun(
	ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession,
) error {
	instanceID := s.InstanceID()

	s.spotSession = sessions[s.SpotSession]
	s.futuresSession = sessions[s.FuturesSession]

	s.spotMarket, _ = s.spotSession.Market(s.Symbol)
	s.futuresMarket, _ = s.futuresSession.Market(s.Symbol)

	var ok bool
	s.binanceFutures, ok = s.futuresSession.Exchange.(*binance.Exchange)
	if !ok {
		return errNotBinanceExchange
	}

	s.binanceSpot, ok = s.spotSession.Exchange.(*binance.Exchange)
	if !ok {
		return errNotBinanceExchange
	}

	if err := s.checkAndFixMarginMode(ctx); err != nil {
		return err
	}

	if err := s.setInitialLeverage(ctx); err != nil {
		return err
	}

	if s.ProfitStats == nil || s.Reset {
		s.ProfitStats = &ProfitStats{
			ProfitStats: types.NewProfitStats(s.Market),
			// when receiving funding fee, the funding fee asset is the quote currency of that market.
			FundingFeeCurrency: s.futuresMarket.QuoteCurrency,
			TotalFundingFee:    fixedpoint.Zero,
			FundingFeeRecords:  nil,
			LastFundingFeeTime: time.Time{},
		}
	}

	// common min quantity
	s.minQuantity = fixedpoint.Max(s.futuresMarket.MinQuantity, s.spotMarket.MinQuantity)

	if s.SpotPosition == nil || s.Reset {
		s.SpotPosition = types.NewPositionFromMarket(s.spotMarket)
	}

	if s.FuturesPosition == nil || s.Reset {
		s.FuturesPosition = types.NewPositionFromMarket(s.futuresMarket)
	}

	if s.NeutralPosition == nil || s.Reset {
		s.NeutralPosition = types.NewPositionFromMarket(s.futuresMarket)
	}

	if s.State == nil || s.Reset {
		s.State = newState()
	}

	if s.ProfitFixerConfig != nil {
		log.Infof("profitFixer is enabled, start fixing with config: %+v", s.ProfitFixerConfig)

		s.SpotPosition = types.NewPositionFromMarket(s.spotMarket)
		s.FuturesPosition = types.NewPositionFromMarket(s.futuresMarket)
		s.ProfitStats.ProfitStats = types.NewProfitStats(s.Market)

		since := s.ProfitFixerConfig.TradesSince.Time()
		now := time.Now()

		spotFixer := common.NewProfitFixer()
		if ss, ok := s.spotSession.Exchange.(types.ExchangeTradeHistoryService); ok {
			spotFixer.AddExchange(s.spotSession.Name, ss)
		}

		if err2 := spotFixer.Fix(ctx, s.Symbol,
			since, now,
			s.ProfitStats.ProfitStats,
			s.SpotPosition); err2 != nil {
			return err2
		}

		futuresFixer := common.NewProfitFixer()
		if ss, ok := s.futuresSession.Exchange.(types.ExchangeTradeHistoryService); ok {
			futuresFixer.AddExchange(s.futuresSession.Name, ss)
		}

		if err2 := futuresFixer.Fix(ctx, s.Symbol,
			since, now,
			s.ProfitStats.ProfitStats,
			s.FuturesPosition); err2 != nil {
			return err2
		}

		bbgo.Notify("Fixed spot position", s.SpotPosition)
		bbgo.Notify("Fixed futures position", s.FuturesPosition)
		bbgo.Notify("Fixed profit stats", s.ProfitStats.ProfitStats)
	}

	if err := s.syncPositionRisks(ctx); err != nil {
		return err
	}

	if s.CloseFuturesPosition && s.Reset {
		return errors.New("reset and closeFuturesPosition can not be used together")
	}

	log.Infof("state: %+v", s.State)
	log.Infof("loaded spot position: %s", s.SpotPosition.String())
	log.Infof("loaded futures position: %s", s.FuturesPosition.String())
	log.Infof("loaded neutral position: %s", s.NeutralPosition.String())

	bbgo.Notify("Spot Position", s.SpotPosition)
	bbgo.Notify("Futures Position", s.FuturesPosition)
	bbgo.Notify("Neutral Position", s.NeutralPosition)
	bbgo.Notify("State: %s", s.State.PositionState.String())

	// sync funding fee txns
	s.syncFundingFeeRecords(ctx, s.ProfitStats.LastFundingFeeTime)

	// TEST CODE:
	// s.syncFundingFeeRecords(ctx, time.Now().Add(-3*24*time.Hour))

	switch s.State.PositionState {
	case PositionClosed:
		// adjust QuoteInvestment according to the available quote balance
		// ONLY when the position is not opening
		if b, ok := s.spotSession.Account.Balance(s.spotMarket.QuoteCurrency); ok {
			originalQuoteInvestment := s.QuoteInvestment

			// adjust available quote with the fee rate
			spotFeeRate := 0.075
			availableQuoteWithoutFee := b.Available.Mul(fixedpoint.NewFromFloat(1.0 - (spotFeeRate * 0.01)))

			s.QuoteInvestment = fixedpoint.Min(availableQuoteWithoutFee, s.QuoteInvestment)

			if originalQuoteInvestment.Compare(s.QuoteInvestment) != 0 {
				log.Infof("adjusted quoteInvestment from %f to %f according to the balance",
					originalQuoteInvestment.Float64(),
					s.QuoteInvestment.Float64(),
				)
			}
		}
	default:
	}

	switch s.State.PositionState {
	case PositionReady:

	case PositionOpening:
		// transfer all base assets from the spot account into the spot account
		if err := s.transferIn(ctx, s.binanceSpot, s.spotMarket.BaseCurrency, fixedpoint.Zero); err != nil {
			log.WithError(err).Errorf("futures asset transfer in error")
		}

	case PositionClosing, PositionClosed:
		// transfer all base assets from the futures account back to the spot account
		if err := s.transferOut(ctx, s.binanceSpot, s.spotMarket.BaseCurrency, fixedpoint.Zero); err != nil {
			log.WithError(err).Errorf("futures asset transfer out error")
		}

	}

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
				return s.transferIn(ctx, s.binanceSpot, s.spotMarket.BaseCurrency, trade.Quantity)
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
			// de-leverage and get the collateral base quantity for transfer
			quantity := trade.Quantity.Div(s.Leverage)

			if err := backoff.RetryGeneral(ctx, func() error {
				return s.transferOut(ctx, s.binanceSpot, s.spotMarket.BaseCurrency, quantity)
			}); err != nil {
				log.WithError(err).Errorf("spot-to-futures transfer in retry failed")
				return
			}

		}
	})

	s.futuresSession.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		s.queryAndDetectPremiumIndex(ctx, s.binanceFutures)
	}))

	s.futuresSession.UserDataStream.OnStart(func() {
		if s.CloseFuturesPosition {

			openOrders, err := s.futuresSession.Exchange.QueryOpenOrders(ctx, s.Symbol)
			if err != nil {
				log.WithError(err).Errorf("query open orders error")
			} else {
				// canceling open orders
				if err = s.futuresSession.Exchange.CancelOrders(ctx, openOrders...); err != nil {
					log.WithError(err).Errorf("query open orders error")
				}
			}

			if err := s.futuresOrderExecutor.ClosePosition(ctx, fixedpoint.One); err != nil {
				log.WithError(err).Errorf("close position error")
			}

			if err := s.resetTransfer(ctx, s.binanceSpot, s.spotMarket.BaseCurrency); err != nil {
				log.WithError(err).Errorf("transfer error")
			}

			if err := s.resetTransfer(ctx, s.binanceSpot, s.spotMarket.QuoteCurrency); err != nil {
				log.WithError(err).Errorf("transfer error")
			}
		}

	})

	if binanceStream, ok := s.futuresSession.UserDataStream.(*binance.Stream); ok {
		binanceStream.OnAccountUpdateEvent(func(e *binance.AccountUpdateEvent) {
			s.handleAccountUpdate(ctx, e)
		})
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				s.syncSpotAccount(ctx)
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				s.syncFuturesAccount(ctx)
			}
		}
	}()

	return nil
}

func (s *Strategy) handleAccountUpdate(ctx context.Context, e *binance.AccountUpdateEvent) {
	switch e.AccountUpdate.EventReasonType {
	case binance.AccountUpdateEventReasonDeposit:
	case binance.AccountUpdateEventReasonWithdraw:
	case binance.AccountUpdateEventReasonFundingFee:
		//  EventBase:{
		// 		Event:ACCOUNT_UPDATE
		// 		Time:1679760000932
		// 	}
		// 	Transaction:1679760000927
		// 	AccountUpdate:{
		// 			EventReasonType:FUNDING_FEE
		// 			Balances:[{
		// 					Asset:USDT
		// 					WalletBalance:56.64251742
		// 					CrossWalletBalance:56.64251742
		// 					BalanceChange:-0.00037648
		// 			}]
		// 		}
		// 	}
		for _, b := range e.AccountUpdate.Balances {
			if b.Asset != s.ProfitStats.FundingFeeCurrency {
				continue
			}
			txnTime := e.EventBase.Time.Time()
			fee := FundingFee{
				Asset:  b.Asset,
				Amount: b.BalanceChange,
				Txn:    e.Transaction,
				Time:   txnTime,
			}
			err := s.ProfitStats.AddFundingFee(fee)
			if err != nil {
				log.WithError(err).Error("unable to add funding fee to profitStats")
				continue
			}

			bbgo.Notify(&fee)
		}

		log.Infof("total collected funding fee: %f %s", s.ProfitStats.TotalFundingFee.Float64(), s.ProfitStats.FundingFeeCurrency)
		bbgo.Sync(ctx, s)

		bbgo.Notify(s.ProfitStats)
	}
}

func (s *Strategy) syncFundingFeeRecords(ctx context.Context, since time.Time) {
	now := time.Now()

	if since.IsZero() {
		since = now.AddDate(0, -3, 0)
	}

	log.Infof("syncing funding fee records from the income history query: %s <=> %s", since, now)

	defer log.Infof("sync funding fee records done")

	q := batch.BinanceFuturesIncomeBatchQuery{
		BinanceFuturesIncomeHistoryService: s.binanceFutures,
	}

	dataC, errC := q.Query(ctx, s.Symbol, binanceapi.FuturesIncomeFundingFee, since, now)
	for {
		select {
		case <-ctx.Done():
			return

		case income, ok := <-dataC:
			if !ok {
				return
			}

			log.Infof("income: %+v", income)
			switch income.IncomeType {
			case binanceapi.FuturesIncomeFundingFee:
				err := s.ProfitStats.AddFundingFee(FundingFee{
					Asset:  income.Asset,
					Amount: income.Income,
					Txn:    income.TranId,
					Time:   income.Time.Time(),
				})
				if err != nil {
					log.WithError(err).Errorf("can not add funding fee record to ProfitStats")
				}
			}

		case err, ok := <-errC:
			if !ok {
				return
			}

			log.WithError(err).Errorf("unable to query futures income history")
			return

		}
	}
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

func (s *Strategy) syncSpotAccount(ctx context.Context) {
	switch s.getPositionState() {
	case PositionOpening:
		s.increaseSpotPosition(ctx)
	case PositionClosing:
		s.syncSpotPosition(ctx)
	}
}

func (s *Strategy) syncFuturesAccount(ctx context.Context) {
	switch s.getPositionState() {
	case PositionOpening:
		s.syncFuturesPosition(ctx)
	case PositionClosing:
		s.reduceFuturesPosition(ctx)
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

	spotBase := s.SpotPosition.GetBase()
	if !s.spotMarket.IsDustQuantity(spotBase, s.SpotPosition.AverageCost) {
		if balance, ok := s.futuresSession.Account.Balance(s.futuresMarket.BaseCurrency); ok && balance.Available.Sign() > 0 {
			if err := backoff.RetryGeneral(ctx, func() error {
				return s.transferOut(ctx, s.binanceSpot, s.spotMarket.BaseCurrency, balance.Available)
			}); err != nil {
				log.WithError(err).Errorf("spot-to-futures transfer in retry failed")
			}
		}
	}

	if futuresBase.Compare(fixedpoint.Zero) < 0 {
		orderPrice := ticker.Buy
		orderQuantity := futuresBase.Abs()
		orderQuantity = fixedpoint.Max(orderQuantity, s.minQuantity)
		orderQuantity = s.futuresMarket.AdjustQuantityByMinNotional(orderQuantity, orderPrice)

		if s.futuresMarket.IsDustQuantity(orderQuantity, orderPrice) {
			submitOrder := types.SubmitOrder{
				Symbol: s.Symbol,
				Side:   types.SideTypeBuy,
				Type:   types.OrderTypeLimitMaker,
				Price:  orderPrice,
				Market: s.futuresMarket,

				// quantity: Cannot be sent with closePosition=true(Close-All)
				// reduceOnly: Cannot be sent with closePosition=true
				ClosePosition: true,
			}

			if _, err := s.futuresOrderExecutor.SubmitOrders(ctx, submitOrder); err != nil {
				log.WithError(err).Errorf("can not submit futures order with close position: %+v", submitOrder)
			}
			return
		}

		submitOrder := types.SubmitOrder{
			Symbol:     s.Symbol,
			Side:       types.SideTypeBuy,
			Type:       types.OrderTypeLimitMaker,
			Quantity:   orderQuantity,
			Price:      orderPrice,
			Market:     s.futuresMarket,
			ReduceOnly: true,
		}
		createdOrders, err := s.futuresOrderExecutor.SubmitOrders(ctx, submitOrder)

		if err != nil {
			log.WithError(err).Errorf("can not submit futures order: %+v", submitOrder)
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

	log.Infof("syncFuturesPosition: position comparision: %s (spot) <=> %s (futures)", spotBase.String(), futuresBase.String())

	if futuresBase.Sign() > 0 {
		// unexpected error
		log.Errorf("unexpected futures position, got positive number (long), expecting negative number (short)")
		return
	}

	// cancel the previous futures order
	_ = s.futuresOrderExecutor.GracefulCancel(ctx)

	// get the latest ticker price
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
	if quoteValue.IsZero() {
		return
	}

	// max futures base position (without negative sign)
	maxFuturesBasePosition := fixedpoint.Min(
		spotBase.Mul(s.Leverage),
		s.State.TotalBaseTransfer.Mul(s.Leverage))

	if maxFuturesBasePosition.IsZero() {
		return
	}

	// if - futures position < max futures position, increase it
	// posDiff := futuresBase.Abs().Sub(maxFuturesBasePosition)
	if futuresBase.Abs().Compare(maxFuturesBasePosition) >= 0 {
		s.setPositionState(PositionReady)

		bbgo.Notify("Position Ready")
		bbgo.Notify("SpotPosition", s.SpotPosition)
		bbgo.Notify("FuturesPosition", s.FuturesPosition)
		bbgo.Notify("NeutralPosition", s.NeutralPosition)

		// DEBUG CODE - triggering closing position automatically
		// s.startClosingPosition()
		return
	}

	orderPrice := ticker.Sell
	diffQuantity := maxFuturesBasePosition.Sub(futuresBase.Neg())

	if diffQuantity.Sign() < 0 {
		log.Errorf("unexpected negative position diff: %s", diffQuantity.String())
		return
	}

	log.Infof("position diff quantity: %s", diffQuantity.String())

	orderQuantity := diffQuantity
	orderQuantity = fixedpoint.Max(diffQuantity, s.minQuantity)
	orderQuantity = s.futuresMarket.AdjustQuantityByMinNotional(orderQuantity, orderPrice)

	if s.futuresMarket.IsDustQuantity(orderQuantity, orderPrice) {
		log.Warnf("unexpected dust quantity, skip futures order with dust quantity %s, market = %+v", orderQuantity.String(), s.futuresMarket)
		return
	}

	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimitMaker,
		Quantity: orderQuantity,
		Price:    orderPrice,
		Market:   s.futuresMarket,
	}
	createdOrders, err := s.futuresOrderExecutor.SubmitOrders(ctx, submitOrder)

	if err != nil {
		log.WithError(err).Errorf("can not submit futures order: %+v", submitOrder)
		return
	}

	log.Infof("created orders: %+v", createdOrders)
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

	log.Infof("syncSpotPosition: spot/futures positions: %s (spot) <=> %s (futures)", spotBase.String(), futuresBase.String())

	if futuresBase.Sign() > 0 {
		// unexpected error
		log.Errorf("unexpected futures position (got positive, expecting negative)")
		return
	}

	_ = s.spotOrderExecutor.GracefulCancel(ctx)

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
		b, ok := s.spotSession.Account.Balance(s.spotMarket.BaseCurrency)
		if !ok {
			log.Warnf("%s balance not found, can not sync spot position", s.spotMarket.BaseCurrency)
			return
		}

		log.Infof("spot balance: %+v", b)

		orderQuantity = fixedpoint.Min(b.Available, orderQuantity)

		// avoid increase the order size
		if s.spotMarket.IsDustQuantity(orderQuantity, orderPrice) {
			log.Infof("skip spot order with dust quantity %s, market=%+v balance=%+v", orderQuantity.String(), s.spotMarket, b)
			return
		}

		submitOrder := types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimitMaker,
			Quantity: orderQuantity,
			Price:    orderPrice,
			Market:   s.spotMarket,
		}
		createdOrders, err := s.spotOrderExecutor.SubmitOrders(ctx, submitOrder)

		if err != nil {
			log.WithError(err).Errorf("can not submit spot order: %+v", submitOrder)
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
	usedQuoteInvestment := s.State.UsedQuoteInvestment
	s.mu.Unlock()

	if usedQuoteInvestment.Compare(s.QuoteInvestment) >= 0 {
		// stop increase the stop position
		return
	}

	_ = s.spotOrderExecutor.GracefulCancel(ctx)

	ticker, err := s.spotSession.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		log.WithError(err).Errorf("can not query ticker")
		return
	}

	leftQuota := s.QuoteInvestment.Sub(usedQuoteInvestment)

	orderPrice := ticker.Buy
	orderQuantity := fixedpoint.Min(s.IncrementalQuoteQuantity, leftQuota).Div(orderPrice)

	log.Infof("initial spot order quantity %s", orderQuantity.String())

	orderQuantity = fixedpoint.Max(orderQuantity, s.minQuantity)
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
		if fundingRate.Compare(s.ShortFundingRate.High) < 0 {
			return false
		}

		log.Infof("funding rate %s is higher than the High threshold %s, start opening position...",
			fundingRate.Percentage(), s.ShortFundingRate.High.Percentage())

		s.startOpeningPosition(types.PositionShort, premiumIndex.Time)
		return true

	case PositionReady:
		if fundingRate.Compare(s.ShortFundingRate.Low) > 0 {
			return false
		}

		log.Infof("funding rate %s is lower than the Low threshold %s, start closing position...",
			fundingRate.Percentage(), s.ShortFundingRate.Low.Percentage())

		bbgo.Notify("%s funding rate %s is lower than the Low threshold %s, start closing position...",
			s.Symbol, fundingRate.Percentage(), s.ShortFundingRate.Low.Percentage())

		holdingPeriod := premiumIndex.Time.Sub(s.State.PositionStartTime)
		if holdingPeriod < time.Duration(s.MinHoldingPeriod) {
			log.Warnf("position holding period %s is less than %s, skip closing", holdingPeriod, s.MinHoldingPeriod.Duration())
			return false
		}

		s.startClosingPosition()
		return true
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

	bbgo.Notify("Start to close position", s.FuturesPosition, s.SpotPosition)

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

func (s *Strategy) allocateOrderExecutor(
	ctx context.Context, session *bbgo.ExchangeSession, instanceID string, position *types.Position,
) *bbgo.GeneralOrderExecutor {
	orderExecutor := bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, position)
	orderExecutor.SetMaxRetries(0)
	orderExecutor.BindEnvironment(s.Environment)
	orderExecutor.Bind()
	orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	orderExecutor.TradeCollector().OnTrade(func(trade types.Trade, _ fixedpoint.Value, _ fixedpoint.Value) {
		s.ProfitStats.AddTrade(trade)

		if profit, netProfit, madeProfit := s.NeutralPosition.AddTrade(trade); madeProfit {
			p := s.NeutralPosition.NewProfit(trade, profit, netProfit)
			s.ProfitStats.AddProfit(p)
		}
	})
	return orderExecutor
}

func (s *Strategy) setInitialLeverage(ctx context.Context) error {
	log.Infof("setting futures leverage to %d", s.Leverage.Int()+1)

	futuresClient := s.binanceFutures.GetFuturesClient()
	req := futuresClient.NewFuturesChangeInitialLeverageRequest()
	req.Symbol(s.Symbol)
	req.Leverage(s.Leverage.Int() + 1)
	resp, err := req.Do(ctx)
	if err != nil {
		return err
	}

	log.Infof("adjusted initial leverage: %+v", resp)
	return nil
}

func (s *Strategy) checkAndFixMarginMode(ctx context.Context) error {
	futuresClient := s.binanceFutures.GetFuturesClient()
	req := futuresClient.NewFuturesGetMultiAssetsModeRequest()
	resp, err := req.Do(ctx)
	if err != nil {
		return err
	}

	if resp.MultiAssetsMargin {
		return nil
	}

	fixReq := futuresClient.NewFuturesChangeMultiAssetsModeRequest()
	fixReq.MultiAssetsMargin(binanceapi.MultiAssetsMarginModeOn)
	fixResp, err := fixReq.Do(ctx)
	if err != nil {
		return err
	}

	log.Infof("changeMultiAssetsMode response: %+v", fixResp)
	return nil
}

func (s *Strategy) syncPositionRisks(ctx context.Context) error {
	futuresClient := s.binanceFutures.GetFuturesClient()
	req := futuresClient.NewFuturesGetPositionRisksRequest()
	req.Symbol(s.Symbol)
	positionRisks, err := req.Do(ctx)
	if err != nil {
		return err
	}

	log.Infof("fetched futures position risks: %+v", positionRisks)

	if len(positionRisks) == 0 {
		s.FuturesPosition.Reset()
		return nil
	}

	for _, positionRisk := range positionRisks {
		if positionRisk.Symbol != s.Symbol {
			continue
		}

		if positionRisk.PositionAmount.IsZero() || positionRisk.EntryPrice.IsZero() {
			continue
		}

		s.FuturesPosition.Base = positionRisk.PositionAmount
		s.FuturesPosition.AverageCost = positionRisk.EntryPrice
		log.Infof("restored futures position from positionRisk: base=%s, average_cost=%s, position_risk=%+v",
			s.FuturesPosition.Base.String(),
			s.FuturesPosition.AverageCost.String(),
			positionRisk)
	}

	return nil
}
