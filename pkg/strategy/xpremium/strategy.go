package xpremium

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xpremium"

var log = logrus.WithField("strategy", ID)

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

	logger        logrus.FieldLogger
	metricsLabels prometheus.Labels

	premiumSession, baseSession, tradingSession *bbgo.ExchangeSession

	// runtime fields
	premiumBook *types.StreamOrderBook
	baseBook    *types.StreamOrderBook

	premiumStream types.Stream
	baseStream    types.Stream

	// add connector manager to manage connectors/streams
	connectorManager *types.ConnectorManager
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

		bidA, askA, okA := s.premiumBook.BestBidAndAsk()
		bidB, askB, okB := s.baseBook.BestBidAndAsk()
		if !okA || !okB {
			continue
		}

		midA := (bidA.Price.Float64() + askA.Price.Float64()) / 2
		midB := (bidB.Price.Float64() + askB.Price.Float64()) / 2
		premium := midA - midB

		var side string
		if s.MinSpread > 0 {
			if premium >= s.MinSpread {
				side = "LONG"
			} else if premium <= -s.MinSpread {
				side = "SHORT"
			}
		}
		if side != "" {
			log.WithFields(logrus.Fields{
				"premium":    premium,
				"minSpread":  s.MinSpread,
				"premiumMid": midA,
				"baseMid":    midB,
				"signalSide": side,
			}).Info("xpremium signal")
			// TODO: integrate order execution here
		} else if time.Since(lastLog) > 5*time.Second {
			lastLog = time.Now()
			log.WithFields(logrus.Fields{
				"premium":    premium,
				"minSpread":  s.MinSpread,
				"premiumMid": midA,
				"baseMid":    midB,
			}).Debug("premium update")
		}
	}
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.PremiumSymbol, types.SubscribeOptions{Interval: "1m"})
}

// Run is only used for back-testing with single session
func (s *Strategy) Run(ctx context.Context, session *bbgo.ExchangeSession) error {
	s.premiumSession = session
	s.baseSession = session
	s.tradingSession = session

	session.MarketDataStream.OnKLineClosed(func(k types.KLine) {

	})

	return nil
}
