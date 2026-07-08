package xfundingv2

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

func getPostionSign(positionType types.PositionType) int {
	switch positionType {
	case types.PositionLong:
		return 1
	case types.PositionShort:
		return -1
	default:
		return 0
	}
}

// MarketSelectionConfig configures the market selection criteria
type MarketSelectionConfig struct {
	FuturesDirection types.PositionType `json:"futuresDirection"`

	MaxHoldingDuration types.Duration `json:"maxHoldingDuration"`
	// TradeBalanceRatio is the ratio of the asset balance to be invested on the spot market.
	TradeBalanceRatio fixedpoint.Value `json:"tradeBalanceRatio"`

	// Minimum annualized funding rate to consider (e.g., 0.10 = 10%)
	// recommended to default to 5%
	MinAnnualizedRate fixedpoint.Value `json:"minAnnualizedRate"`

	MinCollateralRate fixedpoint.Value `json:"minCollateralRate"`

	// Depth
	DepthRatio          fixedpoint.Value `json:"depthRatio"`
	RequiredQuoteVolume fixedpoint.Value `json:"requiredQuoteVolume"`
	// Minimum 24h taker quote volume
	RequiredTakerQuoteVolume24h fixedpoint.Value `json:"requiredTakerVolume24h"`

	// Top N markets by market cap to consider
	TopNCap int `json:"topNCap"`
}

func (c *MarketSelectionConfig) Defaults() {
	if c.FuturesDirection == "" {
		c.FuturesDirection = types.PositionShort
	}
	if c.MaxHoldingDuration == 0 {
		c.MaxHoldingDuration = types.Duration(time.Hour * 48)
	}
	if c.TradeBalanceRatio.IsZero() {
		c.TradeBalanceRatio = fixedpoint.NewFromFloat(0.8)
	}
	if c.MinAnnualizedRate.IsZero() {
		c.MinAnnualizedRate = fixedpoint.NewFromFloat(0.05) // 5%
	}
	if c.MinCollateralRate.IsZero() {
		c.MinCollateralRate = fixedpoint.NewFromFloat(0.95)
	}
	if c.DepthRatio.IsZero() {
		c.DepthRatio = fixedpoint.NewFromFloat(0.05) // 5% depth
	}
	if c.RequiredQuoteVolume.IsZero() {
		c.RequiredQuoteVolume = fixedpoint.NewFromFloat(100000)
	}
	if c.RequiredTakerQuoteVolume24h.IsZero() {
		c.RequiredTakerQuoteVolume24h = fixedpoint.NewFromFloat(100000)
	}
	if c.TopNCap == 0 {
		c.TopNCap = 10
	}
}

// MarketCandidate represents a ranked market candidate
type MarketCandidate struct {
	Symbol string

	PremiumIndex *types.PremiumIndex
	// FundingIntervalHours is the funding interval in hours (e.g., 8 for 8h funding)
	// It's normally 8 for the most Binance perpetuals, but some may have different intervals like 4h.
	// See https://www.binance.com/en/futures/funding-history/perpetual/real-time-funding-rate
	FundingIntervalHours   int
	AnnualizedRate         fixedpoint.Value
	TakerBuyQuoteVolume24h fixedpoint.Value
	InRangeDepth           fixedpoint.Value

	TargetFuturesPosition fixedpoint.Value
	// MinHoldingDuration is the estimated minimum holding interval to break even for this market candidate
	MinHoldingDuration time.Duration
	// MiinHoldingIntervals = MinHoldingDuration / FundingIntervalHours
	MinHoldingIntervals int
}

type FuturesInfoService interface {
	QueryTakerBuySellVolumes(context.Context, string, types.Interval, types.TradeQueryOptions) ([]binanceapi.FuturesTakerBuySellVolume, error)
	QueryPremiumIndex(context.Context, string) (*types.PremiumIndex, error)
	QueryDepth(context.Context, string) (types.SliceOrderBook, int64, error)
	QueryFuturesFundingInfo(context.Context) ([]binanceapi.FuturesFundingInfo, error)
	QueryTicker(context.Context, string) (*types.Ticker, error)
	QueryFuturesAdlRisk(ctx context.Context, symbol string) (map[string]*binanceapi.AdlRisk, error)
}

// MarketSelector selects the best market based on funding rate and liquidity
type MarketSelector struct {
	MarketSelectionConfig
	service FuturesInfoService
	logger  logrus.FieldLogger
}

// NewMarketSelector creates a new MarketSelector
func NewMarketSelector(config MarketSelectionConfig, exchange FuturesInfoService, logger logrus.FieldLogger) *MarketSelector {
	return &MarketSelector{
		MarketSelectionConfig: config,
		service:               exchange,
		logger:                logger,
	}
}

// SelectMarkets returns a list of market candidates that meet the selection criteria
func (s *MarketSelector) SelectMarkets(ctx context.Context, symbols []string) ([]MarketCandidate, error) {
	// Step 1: Query premium indexes for each symbol
	indices, err := queryFundingRates(ctx, s.service, s.logger, symbols)
	if err != nil {
		return nil, err
	}

	// Step 2: Get funding info
	fundingInfos, err := queryFundingInfo(ctx, s.service)
	if err != nil {
		return nil, err
	}

	// Step 3: Get ADL risks
	adlRisks, err := s.service.QueryFuturesAdlRisk(ctx, "")
	if err != nil {
		return nil, err
	}

	// Step 4: Filter candidates
	candidates := make([]MarketCandidate, 0, len(indices))
	// query the volume data
	takerQuoteVolumes := make(map[string]fixedpoint.Value)
	for _, symbol := range symbols {
		takerVals, err := s.service.QueryTakerBuySellVolumes(
			ctx,
			symbol,
			types.Interval1d,
			types.TradeQueryOptions{Limit: 1},
		)
		if err != nil || len(takerVals) == 0 {
			return nil, err
		}
		ticker, err := s.service.QueryTicker(ctx, symbol)
		if err != nil {
			return nil, err
		}
		takerQuoteVolumes[symbol] = takerVals[0].BuyVol.Mul(ticker.Last)
	}
	for _, idx := range indices {
		// filter by funding rate direction
		// short futures -> positive funding rate for funding income
		// long futures -> negative funding rate for funding income
		// position sign * funding rate should be negative for positive funding income
		if getPostionSign(s.FuturesDirection)*idx.LastFundingRate.Sign() > 0 {
			continue
		}

		info, ok := fundingInfos[idx.Symbol]
		if !ok {
			continue
		}
		annualized := AnnualizedRate(idx.LastFundingRate, info.FundingIntervalHours)
		labels := prometheus.Labels{
			"symbol": idx.Symbol,
		}
		annualizedFundingRateMetrics.With(labels).Set(annualized.Float64())
		fundingRateMetrics.With(labels).Set(idx.LastFundingRate.Float64())

		if annualized.Abs().Compare(s.MinAnnualizedRate) < 0 {
			continue
		}

		// exclude symbols with low liquidity in order book
		book, _, err := s.service.QueryDepth(ctx, idx.Symbol)
		if err != nil {
			s.logger.WithError(err).Warnf("failed to query order book for %s, skipping liquidity filter", idx.Symbol)
			continue
		}
		bestBid, ok := book.BestBid()
		if !ok {
			continue
		}
		buyBook := book.SideBook(types.SideTypeBuy)
		inRangeDepth := buyBook.InPriceRange(bestBid.Price, types.SideTypeBuy, s.DepthRatio).SumDepthInQuote()
		if inRangeDepth.Compare(s.RequiredQuoteVolume) <= 0 {
			continue
		}

		takerQuoteVol, ok := takerQuoteVolumes[idx.Symbol]
		if !ok {
			continue
		}
		if takerQuoteVol.Compare(s.RequiredTakerQuoteVolume24h) < 0 {
			continue
		}

		// if the symbol has no ADL risk found, we assume it's low risk.
		adlRisk, found := adlRisks[idx.Symbol]
		if found && adlRisk.RiskLevel != binanceapi.AdlRiskLevelLow {
			continue
		}

		candidates = append(candidates, MarketCandidate{
			Symbol:                 idx.Symbol,
			FundingIntervalHours:   info.FundingIntervalHours,
			PremiumIndex:           idx,
			AnnualizedRate:         annualized,
			TakerBuyQuoteVolume24h: takerQuoteVol,
			InRangeDepth:           inRangeDepth,
		})
	}

	return candidates, nil
}

// queryFundingRates queries funding rates for the given symbols
func queryFundingRates(ctx context.Context, service FuturesInfoService, logger logrus.FieldLogger, symbols []string) ([]*types.PremiumIndex, error) {
	indices := make([]*types.PremiumIndex, 0, len(symbols))

	for _, symbol := range symbols {
		idx, err := service.QueryPremiumIndex(ctx, symbol)
		if err != nil || idx == nil {
			// Log error but continue with other symbols
			logger.WithError(err).Warnf("failed to query latest funding rates for %s", symbol)
			continue
		}
		indices = append(indices, idx)
	}

	return indices, nil
}

func queryFundingInfo(ctx context.Context, service FuturesInfoService) (map[string]*binanceapi.FuturesFundingInfo, error) {
	m := make(map[string]*binanceapi.FuturesFundingInfo)
	fundingInfos, err := service.QueryFuturesFundingInfo(ctx)
	if err != nil {
		return nil, err
	}
	for _, info := range fundingInfos {
		m[info.Symbol] = &info
	}
	return m, nil
}
