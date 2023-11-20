package bybit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
)

const (
	// To maintain aligned fee rates, it's important to update fees frequently.
	feeRatePollingPeriod = time.Minute
)

var (
	pollFeeRateRateLimiter = rate.NewLimiter(rate.Every(10*time.Minute), 1)
)

type symbolFeeDetail struct {
	bybitapi.FeeRate

	BaseCoin  string
	QuoteCoin string
}

// feeRatePoller pulls the specified market data from bbgo QueryMarkets.
type feeRatePoller struct {
	mu     sync.Mutex
	once   sync.Once
	client MarketInfoProvider

	symbolFeeDetail map[string]symbolFeeDetail
}

func newFeeRatePoller(marketInfoProvider MarketInfoProvider) *feeRatePoller {
	return &feeRatePoller{
		client:          marketInfoProvider,
		symbolFeeDetail: map[string]symbolFeeDetail{},
	}
}

func (p *feeRatePoller) Start(ctx context.Context) {
	p.once.Do(func() {
		p.startLoop(ctx)
	})
}

func (p *feeRatePoller) startLoop(ctx context.Context) {
	err := p.poll(ctx)
	if err != nil {
		log.WithError(err).Warn("failed to initialize the fee rate, the ticker is scheduled to update it subsequently")
	}

	ticker := time.NewTicker(feeRatePollingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); !errors.Is(err, context.Canceled) {
				log.WithError(err).Error("context done with error")
			}

			return
		case <-ticker.C:
			if err := p.poll(ctx); err != nil {
				log.WithError(err).Warn("failed to update fee rate")
			}
		}
	}
}

func (p *feeRatePoller) poll(ctx context.Context) error {
	symbolFeeRate, err := p.getAllFeeRates(ctx)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.symbolFeeDetail = symbolFeeRate
	p.mu.Unlock()

	if pollFeeRateRateLimiter.Allow() {
		log.Infof("updated fee rate: %+v", p.symbolFeeDetail)
	}

	return nil
}

func (p *feeRatePoller) Get(symbol string) (symbolFeeDetail, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	fee, found := p.symbolFeeDetail[symbol]
	return fee, found
}

func (e *feeRatePoller) getAllFeeRates(ctx context.Context) (map[string]symbolFeeDetail, error) {
	feeRates, err := e.client.GetAllFeeRates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call get fee rates: %w", err)
	}

	symbolMap := map[string]symbolFeeDetail{}
	for _, f := range feeRates.List {
		if _, found := symbolMap[f.Symbol]; !found {
			symbolMap[f.Symbol] = symbolFeeDetail{FeeRate: f}
		}
	}

	mkts, err := e.client.QueryMarkets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get markets: %w", err)
	}

	// update base coin, quote coin into symbolFeeDetail
	for _, mkt := range mkts {
		feeRate, found := symbolMap[mkt.Symbol]
		if !found {
			continue
		}

		feeRate.BaseCoin = mkt.BaseCurrency
		feeRate.QuoteCoin = mkt.QuoteCurrency

		symbolMap[mkt.Symbol] = feeRate
	}

	// remove trading pairs that are not present in spot market.
	for k, v := range symbolMap {
		if len(v.BaseCoin) == 0 || len(v.QuoteCoin) == 0 {
			log.Debugf("related market not found: %s, skipping the associated trade", k)
			delete(symbolMap, k)
		}
	}

	return symbolMap, nil
}
