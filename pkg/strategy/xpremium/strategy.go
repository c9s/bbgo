package xpremium

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xpremium"

var log = logrus.WithField("strategy", ID)

type BacktestConfig struct {
	BidAskPriceCsv string `json:"bidAskPriceCsv"`
}

type backtestBidAsk struct {
	time                time.Time
	baseAsk, baseBid    fixedpoint.Value
	premiumAsk, premiumBid fixedpoint.Value
}

type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment

	// Symbol is the default trading pair for the trading session (fallback for TradingSymbol)
	Symbol string `json:"symbol"`

	// Premium session & symbol are the leading market to compare
	PremiumSession string `json:"premiumSession"`
	PremiumSymbol  string `json:"premiumSymbol"`

	// Base session & symbol are the lagging market to compare
	BaseSession string `json:"baseSession"`
	BaseSymbol  string `json:"baseSymbol"`

	// Trading session & symbol are where we open LONG/SHORT
	TradingSession string `json:"tradingSession"`
	TradingSymbol  string `json:"tradingSymbol"`

	// MinSpread is the minimum absolute price difference to trigger a signal (premium - base)
	MinSpread float64 `json:"minSpread"`

	// Leverage to set on the trading session (futures)
	Leverage int `json:"leverage"`

	// Quantity is the fixed order size to trade on signal. If zero, sizing will be computed.
	Quantity fixedpoint.Value `json:"quantity"`

	// MaxLossLimit is the maximum quote loss allowed per trade for sizing; if zero, falls back to Quantity/min.
	MaxLossLimit fixedpoint.Value `json:"maxLossLimit"`
	// PriceType selects which price from ticker to use for sizing/validation (maker/taker)
	PriceType types.PriceType `json:"priceType"`

	BacktestConfig *BacktestConfig `json:"backtest,omitempty"`

	logger        logrus.FieldLogger
	metricsLabels prometheus.Labels

	premiumSession, baseSession, tradingSession *bbgo.ExchangeSession
	tradingMarket                               types.Market

	// runtime fields
	premiumBook *types.StreamOrderBook
	baseBook    *types.StreamOrderBook

	premiumStream types.Stream
	baseStream    types.Stream

	// add connector manager to manage connectors/streams
	connectorManager *types.ConnectorManager

	// backtest data map keyed by minute-precision time
	btData map[time.Time]backtestBidAsk
}

func (s *Strategy) ID() string { return ID }

func (s *Strategy) InstanceID() string {
	return strings.Join([]string{ID, s.BaseSession, s.PremiumSession, s.Symbol}, ":")
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}

	s.logger = logrus.WithFields(logrus.Fields{
		"symbol":      s.Symbol,
		"strategy":    ID,
		"strategy_id": s.InstanceID(),
	})

	s.metricsLabels = prometheus.Labels{
		"strategy_type":   ID,
		"strategy_id":     s.InstanceID(),
		"base_session":    s.BaseSession,
		"premium_session": s.PremiumSession,
		"symbol":          s.Symbol,
	}

	// initialize connector manager
	s.connectorManager = types.NewConnectorManager()
	return nil
}

func (s *Strategy) Defaults() error {
	// default trading session to premium session if not specified
	if s.TradingSession == "" {
		s.TradingSession = s.PremiumSession
	}
	// default trading symbol to Symbol, then PremiumSymbol
	if s.TradingSymbol == "" {
		if s.Symbol != "" {
			s.TradingSymbol = s.Symbol
		} else if s.PremiumSymbol != "" {
			s.TradingSymbol = s.PremiumSymbol
		}
	}
	// ensure Symbol has a value for logging/metrics/instance id
	if s.Symbol == "" {
		if s.TradingSymbol != "" {
			s.Symbol = s.TradingSymbol
		} else if s.PremiumSymbol != "" {
			s.Symbol = s.PremiumSymbol
		}
	}
	// default price type
	if s.PriceType == "" {
		s.PriceType = types.PriceTypeMaker
	}
	return nil
}

func (s *Strategy) Validate() error {
	if s.PremiumSession == "" {
		return fmt.Errorf("premiumSession is required")
	}
	if s.BaseSession == "" {
		return fmt.Errorf("baseSession is required")
	}
	if s.PremiumSymbol == "" {
		return fmt.Errorf("premiumSymbol is required")
	}
	if s.BaseSymbol == "" {
		return fmt.Errorf("baseSymbol is required")
	}
	if s.TradingSession == "" {
		return fmt.Errorf("tradingSession is required")
	}
	if s.TradingSymbol == "" {
		return fmt.Errorf("tradingSymbol is required")
	}
	if s.MinSpread <= 0 {
		return fmt.Errorf("minSpread must be greater than 0")
	}
	if s.Leverage < 0 {
		return fmt.Errorf("leverage must be >= 0")
	}
	return nil
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	// Defaults() and Validate() should have been called prior to CrossRun,
	// so we assume required fields are populated here.
	ok := false
	s.premiumSession, ok = sessions[s.PremiumSession]
	if !ok {
		return fmt.Errorf("premium session %s not found", s.PremiumSession)
	}

	s.baseSession, ok = sessions[s.BaseSession]
	if !ok {
		return fmt.Errorf("base session %s not found", s.BaseSession)
	}

	s.tradingSession, ok = sessions[s.TradingSession]
	if !ok {
		return fmt.Errorf("trading session %s not found", s.TradingSession)
	}
	tradingSymbol := s.TradingSymbol

	// initialize common.Strategy with trading session and market to use Position, ProfitStats and GeneralOrderExecutor
	tradingMarket, ok := s.tradingSession.Market(tradingSymbol)
	if !ok {
		return fmt.Errorf("trading session market %s is not defined", tradingSymbol)
	}
	// keep a reference to trading market
	s.tradingMarket = tradingMarket

	// Initialize the core strategy components (Position, ProfitStats, GeneralOrderExecutor)
	s.Strategy.Initialize(ctx, s.Environment, s.tradingSession, tradingMarket, ID, s.InstanceID())

	// set leverage if configured and supported
	if s.Leverage > 0 {
		if riskSvc, ok := s.tradingSession.Exchange.(types.ExchangeRiskService); ok {
			if err := riskSvc.SetLeverage(ctx, tradingSymbol, s.Leverage); err != nil {
				s.logger.WithError(err).Warnf("failed to set leverage to %d on %s", s.Leverage, tradingSymbol)
			} else {
				s.logger.Infof("leverage set to %d on %s", s.Leverage, tradingSymbol)
			}
		} else {
			s.logger.Infof("exchange of trading session %s does not support leverage API", s.TradingSession)
		}
	}

	// allocate isolated public streams for books and bind StreamBooks
	premiumStream := bbgo.NewBookStream(s.premiumSession, s.PremiumSymbol)
	baseStream := bbgo.NewBookStream(s.baseSession, s.BaseSymbol)
	s.premiumStream, s.baseStream = premiumStream, baseStream

	s.premiumBook = types.NewStreamBook(s.PremiumSymbol, s.premiumSession.ExchangeName)
	s.baseBook = types.NewStreamBook(s.BaseSymbol, s.baseSession.ExchangeName)
	s.premiumBook.BindStream(premiumStream)
	s.baseBook.BindStream(baseStream)

	// register streams into connector manager and connect them via connector manager
	s.connectorManager.Add(premiumStream)
	s.connectorManager.Add(baseStream)

	if err := s.connectorManager.Connect(ctx); err != nil {
		s.logger.WithError(err).Error("connector manager connect error")
		return err
	}

	// wait for both sessions' user data streams to be authenticated before starting the premium worker
	group := types.NewConnectivityGroup(
		s.premiumSession.UserDataConnectivity,
		s.baseSession.UserDataConnectivity,
	)

	go func() {
		s.logger.Infof("waiting for authentication of premium and base sessions...")
		select {
		case <-ctx.Done():
			return
		case <-group.AllAuthedC(ctx):
		}

		s.logger.Infof("both premium and base sessions authenticated, starting premium worker")

		s.premiumWorker(ctx)
	}()

	return nil
}

// computeSpreads implements the bid-ask comparison algorithm:
// premium = premiumBid - baseAsk
// discount = premiumAsk - baseBid
func (s *Strategy) computeSpreads(pBid, pAsk, bBid, bAsk types.PriceVolume) (premium, discount float64) {
	premium = pBid.Price.Sub(bAsk.Price).Float64()
	discount = pAsk.Price.Sub(bBid.Price).Float64()
	return premium, discount
}

// compareBooks fetches best bid/ask from both books and returns spreads
func (s *Strategy) compareBooks() (premium, discount float64, pBid, pAsk, bBid, bAsk types.PriceVolume, ok bool) {
	bidA, askA, okA := s.premiumBook.BestBidAndAsk()
	bidB, askB, okB := s.baseBook.BestBidAndAsk()
	if !okA || !okB {
		return 0, 0, types.PriceVolume{}, types.PriceVolume{}, types.PriceVolume{}, types.PriceVolume{}, false
	}
	prem, disc := s.computeSpreads(bidA, askA, bidB, askB)
	return prem, disc, bidA, askA, bidB, askB, true
}

// decideSignal determines LONG when premium >= MinSpread, SHORT when discount <= -MinSpread
func (s *Strategy) decideSignal(premium, discount float64) types.SideType {
	if s.MinSpread <= 0 {
		return ""
	}
	if premium >= s.MinSpread {
		return types.SideTypeBuy
	}
	if discount <= -s.MinSpread {
		return types.SideTypeSell
	}
	return ""
}

// getPrev15mStop determines stop loss from the previous closed 15m kline:
// - LONG: use previous low
// - SHORT: use previous high
func (s *Strategy) getPrev15mStop(ctx context.Context, side types.SideType) (fixedpoint.Value, error) {
	interval := types.Interval15m
	now := time.Now()
	end := now
	// request recent klines; many exchanges honor Limit
	klines, err := s.tradingSession.Exchange.QueryKLines(ctx, s.TradingSymbol, interval, types.KLineQueryOptions{
		EndTime: &end,
		Limit:   2,
	})
	if err != nil || len(klines) == 0 {
		return fixedpoint.Zero, fmt.Errorf("query 15m klines error: %w", err)
	}
	var prev types.KLine
	if len(klines) >= 2 {
		prev = klines[len(klines)-2]
	} else {
		prev = klines[len(klines)-1]
	}
	if side == types.SideTypeBuy {
		return prev.GetLow(), nil
	}
	return prev.GetHigh(), nil
}

// calculatePositionSize sizes order using MaxLossLimit and stop loss like tradingdesk
func (s *Strategy) calculatePositionSize(ctx context.Context, side types.SideType, stopLoss fixedpoint.Value) (fixedpoint.Value, error) {
	// If MaxLossLimit is zero or stopLoss invalid, fallback to configured Quantity or min
	if s.MaxLossLimit.IsZero() || stopLoss.IsZero() {
		if s.Quantity.Sign() > 0 {
			return s.Quantity, nil
		}
		return s.tradingMarket.MinQuantity, nil
	}

	ticker, err := s.tradingSession.Exchange.QueryTicker(ctx, s.TradingSymbol)
	if err != nil {
		return fixedpoint.Zero, err
	}
	currentPrice := s.PriceType.GetPrice(ticker, side)
	if currentPrice.IsZero() {
		currentPrice = ticker.GetValidPrice()
	}
	if currentPrice.IsZero() {
		return fixedpoint.Zero, fmt.Errorf("invalid current price")
	}

	// validate stop relative to price
	if side == types.SideTypeBuy {
		if stopLoss.Compare(currentPrice) >= 0 {
			return fixedpoint.Zero, fmt.Errorf("stop loss must be below current price for long")
		}
	} else if side == types.SideTypeSell {
		if stopLoss.Compare(currentPrice) <= 0 {
			return fixedpoint.Zero, fmt.Errorf("stop loss must be above current price for short")
		}
	}

	var riskPerUnit fixedpoint.Value
	if side == types.SideTypeBuy {
		riskPerUnit = currentPrice.Sub(stopLoss)
	} else {
		riskPerUnit = stopLoss.Sub(currentPrice)
	}
	if riskPerUnit.Sign() <= 0 {
		return fixedpoint.Zero, fmt.Errorf("invalid risk per unit")
	}
	maxQtyByRisk := s.MaxLossLimit.Div(riskPerUnit)

	// balance constraint
	account := s.tradingSession.GetAccount()
	var maxQtyByBalance fixedpoint.Value
	if s.tradingSession.Futures {
		quoteBal, ok := account.Balance(s.tradingMarket.QuoteCurrency)
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("no %s balance", s.tradingMarket.QuoteCurrency)
		}
		maxQtyByBalance = quoteBal.Available.Mul(fixedpoint.NewFromInt(int64(s.Leverage))).Div(currentPrice)
	} else {
		if side == types.SideTypeBuy {
			quoteBal, ok := account.Balance(s.tradingMarket.QuoteCurrency)
			if !ok {
				return fixedpoint.Zero, fmt.Errorf("no %s balance", s.tradingMarket.QuoteCurrency)
			}
			maxQtyByBalance = quoteBal.Available.Div(currentPrice)
		} else {
			baseBal, ok := account.Balance(s.tradingMarket.BaseCurrency)
			if !ok {
				return fixedpoint.Zero, fmt.Errorf("no %s balance", s.tradingMarket.BaseCurrency)
			}
			maxQtyByBalance = baseBal.Available
		}
	}

	qty := fixedpoint.Min(maxQtyByRisk, maxQtyByBalance)
	// apply market constraints
	qty = s.tradingMarket.TruncateQuantity(qty)
	if qty.Compare(s.tradingMarket.MinQuantity) < 0 {
		qty = s.tradingMarket.MinQuantity
	}
	qty = s.tradingMarket.AdjustQuantityByMinNotional(qty, currentPrice)
	return qty, nil
}

// executeSignal submits a market order and a stop market order as stop loss
func (s *Strategy) executeSignal(ctx context.Context, side types.SideType) error {
	if side == "" || s.OrderExecutor == nil {
		return nil
	}

	// derive stop loss from previous 15m kline
	stopLoss, err := s.getPrev15mStop(ctx, side)
	if err != nil {
		s.logger.WithError(err).Warn("failed to get 15m stop, fallback to quantity only")
	}

	qty, qerr := s.calculatePositionSize(ctx, side, stopLoss)
	if qerr != nil {
		s.logger.WithError(qerr).Warn("position sizing failed, fallback to min qty")
		qty = s.tradingMarket.MinQuantity
	}

	order := types.SubmitOrder{
		Symbol:   s.TradingSymbol,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: qty,
		Market:   s.tradingMarket,
	}
	_, err = s.OrderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		return err
	}

	// place stop loss if we have a valid stop
	if stopLoss.Sign() > 0 {
		stopOrder := types.SubmitOrder{
			Market:        s.tradingMarket,
			Symbol:        s.TradingSymbol,
			Side:          side.Reverse(),
			Type:          types.OrderTypeStopMarket,
			StopPrice:     stopLoss,
			ClosePosition: true,
			ReduceOnly:    false,
		}
		if _, err := s.OrderExecutor.SubmitOrders(ctx, stopOrder); err != nil {
			s.logger.WithError(err).Warn("submit stop loss order failed")
		}
	}

	return nil
}

func (s *Strategy) premiumWorker(ctx context.Context) {
	log := s.logger.WithField("worker", "premium")
	var lastLog time.Time
	for {
		select {
		case <-ctx.Done():
			log.Info("context canceled, stop premium worker")
			return
		case <-s.premiumBook.C:
			// fallthrough to evaluate when either book updates
		case <-s.baseBook.C:
		}

		bookA := s.premiumBook
		bookB := s.baseBook
		if ok, err := bookA.IsValid(); !ok || err != nil {
			continue
		}

		if ok, err := bookB.IsValid(); !ok || err != nil {
			continue
		}

		premium, discount, bidA, askA, bidB, askB, ok := s.compareBooks()
		if !ok {
			continue
		}

		side := s.decideSignal(premium, discount)
		if side != "" {
			log.WithFields(logrus.Fields{
				"premium":   premium,
				"discount":  discount,
				"minSpread": s.MinSpread,
				"pBid":      bidA.Price.Float64(),
				"pAsk":      askA.Price.Float64(),
				"bBid":      bidB.Price.Float64(),
				"bAsk":      askB.Price.Float64(),
				"signal":    side.String(),
			}).Info("xpremium signal")

			// simple position alignment: avoid re-entering same direction immediately
			if (side == types.SideTypeBuy && s.Position != nil && s.Position.IsLong()) ||
				(side == types.SideTypeSell && s.Position != nil && s.Position.IsShort()) {
				continue
			}

			if err := s.executeSignal(ctx, side); err != nil {
				log.WithError(err).Error("executeSignal error")
			}
		} else if time.Since(lastLog) > 5*time.Second {
			lastLog = time.Now()
			log.WithFields(logrus.Fields{
				"premium":   premium,
				"discount":  discount,
				"minSpread": s.MinSpread,
			}).Debug("premium update")
		}
	}
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.PremiumSymbol, types.SubscribeOptions{Interval: "1m"})
}

// loadBacktestCSV loads bid/ask CSV and stores by minute-precision time
func (s *Strategy) loadBacktestCSV(path string) error {
	if path == "" {
		return fmt.Errorf("backtest.bidAskPriceCsv is empty")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return err
	}
	if len(records) < 2 {
		return fmt.Errorf("no data in csv: %s", path)
	}
	// initialize map
	if s.btData == nil {
		s.btData = make(map[time.Time]backtestBidAsk, len(records)-1)
	}
	// skip header (records[0])
	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) < 5 {
			continue
		}
		// parse time in local timezone
		t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(row[0]), time.Local)
		if err != nil {
			return fmt.Errorf("parse time at row %d: %w", i+1, err)
		}
		// columns: 2 base ask, 3 base bid, 4 premium ask, 5 premium bid
		pa := strings.TrimSpace(row[3])
		pb := strings.TrimSpace(row[4])
		ba := strings.TrimSpace(row[1])
		bb := strings.TrimSpace(row[2])

		parse := func(sv string) (fixedpoint.Value, error) {
			// allow integer-like numbers
			if strings.Contains(sv, ".") {
				f, err := strconv.ParseFloat(sv, 64)
				if err != nil { return fixedpoint.Zero, err }
				return fixedpoint.NewFromFloat(f), nil
			}
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				// try float as fallback
				f, ferr := strconv.ParseFloat(sv, 64)
				if ferr != nil { return fixedpoint.Zero, err }
				return fixedpoint.NewFromFloat(f), nil
			}
			return fixedpoint.NewFromInt(iv), nil
		}

		pAsk, err := parse(pa)
		if err != nil { return fmt.Errorf("parse premium ask at row %d: %w", i+1, err) }
		pBid, err := parse(pb)
		if err != nil { return fmt.Errorf("parse premium bid at row %d: %w", i+1, err) }
		bAsk, err := parse(ba)
		if err != nil { return fmt.Errorf("parse base ask at row %d: %w", i+1, err) }
		bBid, err := parse(bb)
		if err != nil { return fmt.Errorf("parse base bid at row %d: %w", i+1, err) }

		key := t.Truncate(time.Minute)
		s.btData[key] = backtestBidAsk{time: key, baseAsk: bAsk, baseBid: bBid, premiumAsk: pAsk, premiumBid: pBid}
	}
	return nil
}

func (s *Strategy) lookupBacktestAt(t time.Time) (backtestBidAsk, bool) {
	if s.btData == nil { return backtestBidAsk{}, false }
	key := t.Truncate(time.Minute)
	if v, ok := s.btData[key]; ok { return v, true }
	return backtestBidAsk{}, false
}

// Run is only used for back-testing with single session
func (s *Strategy) Run(ctx context.Context, session *bbgo.ExchangeSession) error {
	// in backtest, we run on a single session; use premium as trading session
	s.premiumSession = session
	s.baseSession = session
	s.tradingSession = session

	// derive trading symbol defaults
	if s.TradingSymbol == "" {
		if s.Symbol != "" { s.TradingSymbol = s.Symbol } else { s.TradingSymbol = s.PremiumSymbol }
	}
	if s.Symbol == "" { s.Symbol = s.TradingSymbol }

	// market and common strategy init
	market, ok := session.Market(s.TradingSymbol)
	if !ok { return fmt.Errorf("market %s not found in backtest session", s.TradingSymbol) }
	s.tradingMarket = market
	s.Strategy.Initialize(ctx, s.Environment, session, market, ID, s.InstanceID())

	// load csv if configured
	if s.BacktestConfig != nil && s.BacktestConfig.BidAskPriceCsv != "" {
		if err := s.loadBacktestCSV(s.BacktestConfig.BidAskPriceCsv); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("backtest config or csv path not provided")
	}

	// subscribe to klines for time alignment; assume 1m unless different backtest interval
	session.MarketDataStream.OnKLineClosed(func(k types.KLine) {
		// only act on our trading symbol
		if k.Symbol != s.TradingSymbol { return }

		// match kline time with csv time; prefer k.EndTime
		if rec, ok := s.lookupBacktestAt(k.EndTime.Time()); ok {
			// rebuild best bid/ask as PriceVolume
			pBid := types.PriceVolume{Price: rec.premiumBid, Volume: fixedpoint.One}
			pAsk := types.PriceVolume{Price: rec.premiumAsk, Volume: fixedpoint.One}
			bBid := types.PriceVolume{Price: rec.baseBid, Volume: fixedpoint.One}
			bAsk := types.PriceVolume{Price: rec.baseAsk, Volume: fixedpoint.One}

			premium, discount := s.computeSpreads(pBid, pAsk, bBid, bAsk)
			side := s.decideSignal(premium, discount)
			if side != "" {
				// synchronous execution in backtest
				if err := s.executeSignal(ctx, side); err != nil {
					s.logger.WithError(err).Error("backtest executeSignal error")
				}
			}
			return
		}
		// fallback to StartTime
		if rec, ok := s.lookupBacktestAt(k.StartTime.Time()); ok {
			pBid := types.PriceVolume{Price: rec.premiumBid, Volume: fixedpoint.One}
			pAsk := types.PriceVolume{Price: rec.premiumAsk, Volume: fixedpoint.One}
			bBid := types.PriceVolume{Price: rec.baseBid, Volume: fixedpoint.One}
			bAsk := types.PriceVolume{Price: rec.baseAsk, Volume: fixedpoint.One}
			premium, discount := s.computeSpreads(pBid, pAsk, bBid, bAsk)
			side := s.decideSignal(premium, discount)
			if side != "" {
				if err := s.executeSignal(ctx, side); err != nil {
					s.logger.WithError(err).Error("backtest executeSignal error")
				}
			}
		}
	})

	return nil
}
