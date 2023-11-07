package bybit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/util"
)

const (
	// To maintain aligned fee rates, it's important to update fees frequently.
	feeRatePollingPeriod = time.Minute
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
	ticker := time.NewTicker(feeRatePollingPeriod)
	defer ticker.Stop()

	// Make sure the first poll should succeed by retrying with a shorter period.
	_ = util.Retry(ctx, util.InfiniteRetry, 30*time.Second,
		func() error { return p.poll(ctx) },
		func(e error) { log.WithError(e).Warn("failed to update fee rate") })

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

	return nil
}

func (p *feeRatePoller) Get(symbol string) (symbolFeeDetail, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	fee, ok := p.symbolFeeDetail[symbol]
	if !ok {
		return symbolFeeDetail{}, fmt.Errorf("%s fee rate not found", symbol)
	}
	return fee, nil
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
