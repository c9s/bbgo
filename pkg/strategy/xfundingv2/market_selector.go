package xfundingv2

import (
	"context"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

// MarketSelectionConfig configures the market selection criteria
type MarketSelectionConfig struct {

	// Minimum annualized funding rate to consider (e.g., 0.10 = 10%)
	MinAnnualizedRate fixedpoint.Value `json:"minAnnualizedRate"`

	// Minimum 24h quote volume
	MinVolume24h fixedpoint.Value `json:"minVolume24h"`

	MinCollateralRate fixedpoint.Value `json:"minCollateralRate"`

	// Depth
	DepthRatio          fixedpoint.Value `json:"depthRatio"`
	RequiredQuoteVolume fixedpoint.Value `json:"requiredQuoteVolume"`

	RequiredTakerQuoteVolume24h fixedpoint.Value `json:"requiredTakerVolume24h"`

	// Top N markets by market cap to consider
	TopNCap int `json:"topNCap"`
}

// MarketCandidate represents a ranked market candidate
type MarketCandidate struct {
	Symbol string `json:"symbol"`

	LastFundingRate        fixedpoint.Value `json:"lastFundingRate"`
	AnnualizedRate         fixedpoint.Value `json:"annualizedRate"`
	TakerBuyQuoteVolume24h fixedpoint.Value `json:"takerBuyQuoteVolume24h"`
	InRangeDepth           fixedpoint.Value `json:"inRangeDepth"`

	Score fixedpoint.Value `json:"score"` // maybe later
}

// MarketSelector selects the best market based on funding rate and liquidity
type MarketSelector struct {
	*MarketSelectionConfig
	binanceFutures *binance.Exchange
	orderBooks     map[string]*types.StreamOrderBook
	logger         logrus.FieldLogger
}

// NewMarketSelector creates a new MarketSelector
func NewMarketSelector(config *MarketSelectionConfig, exchange *binance.Exchange, orderBooks map[string]*types.StreamOrderBook, logger logrus.FieldLogger) *MarketSelector {
	if config == nil {
		config = &MarketSelectionConfig{}
	}
	return &MarketSelector{
		MarketSelectionConfig: config,
		binanceFutures:        exchange,
		orderBooks:            orderBooks,
		logger:                logger,
	}
}

// RankMarkets returns a ranked list of market candidates
func (m *MarketSelector) RankMarkets(ctx context.Context, symbols []string) ([]MarketCandidate, error) {
	// Step 1: Query premium indexes for each symbol
	indices, err := m.queryFundingRates(ctx, symbols)
	if err != nil {
		return nil, err
	}

	// Step 2: Get funding info
	fundingInfos, err := m.queryFundingInfo(ctx)
	if err != nil {
		return nil, err
	}

	// Step 3: Filter candidates
	candidates := make([]MarketCandidate, 0, len(indices))
	// query the volume data
	takerQuoteVolumes := make(map[string]fixedpoint.Value)
	for _, symbol := range symbols {
		takerVals, err := m.binanceFutures.QueryTakerBuySellVolumes(
			ctx,
			symbol,
			types.Interval1d,
			types.TradeQueryOptions{Limit: 1},
		)
		if err != nil || len(takerVals) == 0 {
			return nil, err
		}
		ticker, err := m.binanceFutures.QueryTicker(ctx, symbol)
		if err != nil {
			return nil, err
		}
		takerQuoteVolumes[symbol] = takerVals[0].BuyVol.Mul(ticker.Last)
	}
	for _, idx := range indices {
		// Only consider positive funding rates above threshold
		if idx.LastFundingRate.Sign() <= 0 {
			continue
		}

		info, ok := fundingInfos[idx.Symbol]
		if !ok {
			continue
		}
		annualized := AnnualizedRate(idx.LastFundingRate, info.FundingIntervalHours)

		if annualized.Compare(m.MinAnnualizedRate) < 0 {
			continue
		}

		// exclude symbols with low liquidity in order book
		book, ok := m.orderBooks[idx.Symbol]
		if !ok {
			continue
		}
		bestBid, ok := book.BestBid()
		if !ok {
			continue
		}
		buyBook := book.SideBook(types.SideTypeBuy)
		inRangeDepth := buyBook.InPriceRange(bestBid.Price, types.SideTypeBuy, m.DepthRatio).SumDepthInQuote()
		if inRangeDepth.Compare(m.RequiredQuoteVolume) <= 0 {
			continue
		}

		takerQuoteVol, ok := takerQuoteVolumes[idx.Symbol]
		if !ok {
			continue
		}
		if takerQuoteVol.Compare(m.RequiredTakerQuoteVolume24h) < 0 {
			continue
		}

		candidates = append(candidates, MarketCandidate{
			Symbol:                 idx.Symbol,
			LastFundingRate:        idx.LastFundingRate,
			AnnualizedRate:         annualized,
			TakerBuyQuoteVolume24h: takerQuoteVol,
			InRangeDepth:           inRangeDepth,
		})
	}

	// TODO: compute composite score and sort candidates

	return candidates, nil
}

// queryFundingRates queries funding rates for the given symbols
func (m *MarketSelector) queryFundingRates(ctx context.Context, symbols []string) ([]*types.PremiumIndex, error) {
	indices := make([]*types.PremiumIndex, 0, len(symbols))

	for _, symbol := range symbols {
		idx, err := m.binanceFutures.QueryPremiumIndex(ctx, symbol)
		if err != nil || idx == nil {
			// Log error but continue with other symbols
			m.logger.WithError(err).Warnf("failed to query latest funding rates for %s", symbol)
			continue
		}
		indices = append(indices, idx)
	}

	return indices, nil
}

func (s *MarketSelector) queryFundingInfo(ctx context.Context) (map[string]*binanceapi.FuturesFundingInfo, error) {
	m := make(map[string]*binanceapi.FuturesFundingInfo)
	fundingInfos, err := s.binanceFutures.QueryFuturesFundingInfo(ctx)
	if err != nil {
		return nil, err
	}
	for _, info := range fundingInfos {
		m[info.Symbol] = &info
	}
	return m, nil
}

// calculateScores calculates composite scores for ranking
// Score = AnnualizedRate * 0.6 + NormalizedVolume * 0.4
func (m *MarketSelector) calculateScores(candidates []MarketCandidate) {
	if len(candidates) == 0 {
		return
	}

	// Find max values for normalization
	var maxRate, maxVolume fixedpoint.Value
	for _, c := range candidates {
		if c.AnnualizedRate.Compare(maxRate) > 0 {
			maxRate = c.AnnualizedRate
		}
		if c.TakerBuyQuoteVolume24h.Compare(maxVolume) > 0 {
			maxVolume = c.TakerBuyQuoteVolume24h
		}
	}

	// Calculate normalized scores
	for i := range candidates {
		var normalizedRate, normalizedVolume fixedpoint.Value

		if !maxRate.IsZero() {
			normalizedRate = candidates[i].AnnualizedRate.Div(maxRate)
		}
		if !maxVolume.IsZero() {
			normalizedVolume = candidates[i].TakerBuyQuoteVolume24h.Div(maxVolume)
		}

		// Weighted score: rate (60%) + volume (40%)
		rateScore := normalizedRate.Mul(fixedpoint.NewFromFloat(0.6))
		volumeScore := normalizedVolume.Mul(fixedpoint.NewFromFloat(0.4))

		candidates[i].Score = rateScore.Add(volumeScore)
	}
}
