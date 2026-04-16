package xfundingv2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datasource/coinmarketcap"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

const ID = "xfundingv2"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment

	// Session configuration
	SpotSession    string `json:"spotSession"`
	FuturesSession string `json:"futuresSession"`

	// CandidateSymbols is the list of symbols to consider for selection
	// IMPORTANT: xfundingv2 is now assuming trading on U-major pairs
	CandidateSymbols []string `json:"candidateSymbols"`
	// Market selection criteria
	MarketSelectionConfig *MarketSelectionConfig `json:"marketSelection,omitempty"`

	depthBooks                              map[string]*types.StreamOrderBook
	spotMarkets, futuresMarkets             types.MarketMap
	spotSession, futuresSession             *bbgo.ExchangeSession
	spotOrderExecutor, futuresOrderExecutor *bbgo.GeneralOrderExecutor

	coinmarketcapClient *coinmarketcap.DataSource
	mu                  sync.Mutex
	currentTime         time.Time

	logger logrus.FieldLogger
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	symbols := strings.Join(s.CandidateSymbols, "_")
	return fmt.Sprintf("%s-%s", ID, symbols)
}

func (s *Strategy) Defaults() error {
	return nil
}

func (s *Strategy) Initialize() error {
	s.logger = logrus.WithFields(logrus.Fields{
		"strategy":    ID,
		"strategy_id": s.InstanceID(),
	})

	if apiKey := os.Getenv("COINMARKETCAP_API_KEY"); apiKey == "" {
		s.logger.Warn("CoinMarketCap API key not set, top cap market filtering will be disabled")
	} else {
		s.coinmarketcapClient = coinmarketcap.New(apiKey)
	}
	return nil
}

func (s *Strategy) Validate() error {
	if s.MarketSelectionConfig == nil {
		return errors.New("marketSelection config is required")
	}
	if len(s.CandidateSymbols) == 0 {
		return errors.New("candidateSymbols is required")
	}

	return nil
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	spotSession := sessions[s.SpotSession]
	futuresSession := sessions[s.FuturesSession]

	for _, symbol := range s.CandidateSymbols {
		spotSession.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: types.Interval1m})
		spotSession.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{})
		futuresSession.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: types.Interval1m})
		futuresSession.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{})
	}
}

func (s *Strategy) CrossRun(
	ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession,
) error {
	s.spotSession = sessions[s.SpotSession]
	s.futuresSession = sessions[s.FuturesSession]

	if s.futuresSession == nil {
		return fmt.Errorf("futures session %s not found", s.FuturesSession)
	}
	if s.spotSession == nil {
		return fmt.Errorf("spot session %s not found", s.SpotSession)
	}

	spotMarkets, err := s.spotSession.Exchange.QueryMarkets(ctx)
	if err != nil {
		return fmt.Errorf("failed to query spot markets: %w", err)
	}
	s.spotMarkets = spotMarkets
	futuresMarkets, err := s.futuresSession.Exchange.QueryMarkets(ctx)
	if err != nil {
		return fmt.Errorf("failed to query futures markets: %w", err)
	}
	s.futuresMarkets = futuresMarkets

	// static filters
	var candidateSymbols []string
	// 1. should be listed on both spot and futures
	candidateSymbols = s.filterMarketBothListed(s.CandidateSymbols)
	// 2. filter by collateral rate
	candidateSymbols = s.filterMarketCollateralRate(ctx, candidateSymbols)
	// 3. filter by top N market cap
	candidateSymbols = s.filterMarketByCapSize(ctx, candidateSymbols)

	// initialize depth books for model selection
	for _, symbol := range candidateSymbols {
		book := types.NewStreamBook(symbol, s.futuresSession.ExchangeName)
		book.BindStream(s.futuresSession.MarketDataStream)
		s.depthBooks[symbol] = book
	}

	return nil
}

func (s *Strategy) arbitrage(tickTime time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.currentTime.IsZero() && tickTime.Before(s.currentTime) {
		return
	}
	s.currentTime = tickTime

	// start arbitrage round
}

// static market filters
func (s *Strategy) filterMarketByCapSize(ctx context.Context, symbols []string) []string {
	if s.coinmarketcapClient == nil {
		return symbols
	}
	topAssets, err := s.queryTopCapAssets(ctx)
	if err != nil {
		return symbols
	}
	var candidateSymbols []string
	for _, symbol := range symbols {
		market, ok := s.futuresMarkets[symbol]
		if !ok {
			continue
		}
		if _, found := topAssets[market.BaseCurrency]; found {
			candidateSymbols = append(candidateSymbols, symbol)
		}
	}
	return candidateSymbols
}

func (s *Strategy) filterMarketBothListed(symbols []string) []string {
	var candidateSymbols []string
	for _, symbol := range symbols {
		_, spotOk := s.spotMarkets[symbol]
		_, futuresOk := s.futuresMarkets[symbol]
		if spotOk && futuresOk {
			candidateSymbols = append(candidateSymbols, symbol)
		} else {
			s.logger.Infof("skipping %s as it's not listed on both spot and futures", symbol)
		}
	}
	return candidateSymbols
}

func (s *Strategy) filterMarketCollateralRate(ctx context.Context, symbols []string) []string {
	var markets []types.Market
	for _, symbol := range symbols {
		market, ok := s.futuresMarkets[symbol]
		if !ok {
			continue
		}
		markets = append(markets, market)
	}
	var baseAssets []string
	for _, market := range markets {
		baseAssets = append(baseAssets, market.BaseCurrency)
	}
	collateralRates, err := queryPortfolioModeCollateralRates(ctx, baseAssets)
	if err != nil {
		s.logger.WithError(err).Warn("failed to query collateral rates, skipping collateral rate filter")
		return symbols
	}
	var candidateSymbols []string
	for _, market := range markets {
		rate, ok := collateralRates[market.BaseCurrency]
		if !ok {
			continue
		}
		if rate.Compare(s.MarketSelectionConfig.MinCollateralRate) >= 0 {
			candidateSymbols = append(candidateSymbols, market.Symbol)
		} else {
			s.logger.Infof("skipping %s due to low collateral rate: %s", market.Symbol, rate.String())
		}
	}
	return candidateSymbols
}
