package service

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pricesolver"
	"github.com/c9s/bbgo/pkg/types"
)

const interestPriceQuote = "USDT"

// interestPriceMaxRetries is the number of retries of a failed kline page query
const interestPriceMaxRetries = 2

// interestPriceRecentHours is how long an hour without a published kline is
// treated as "not published yet" (an error, retried by the next sync) instead
// of "no kline" (price 0, kept).
const interestPriceRecentHours = 2

// hourlyOpenPricer prices interest rows of one asset at the time they were charged,
// using the open price of the 1h kline that contains the row time.
// It is created once per sync run. A zero price means unknown.
//
// It is built on the same blocks as the profit fixer's fee prices:
// batch.KLineBatchQuery pages the klines, and pricesolver.SimplePriceSolver turns
// the open of the asset's USDT pair into a USDT price, so an inverse pair such
// as USDTTRY works too. Unlike the profit fixer (previous day's 1d close), the
// price is the open of the charge hour.
//
// priceAt returns an error when the price is unknown only for now (a failed
// kline query, or a kline that is not published yet). The insert then fails,
// so the row is not stored with price 0 and the next sync resumes before it.
type hourlyOpenPricer struct {
	ex     types.Exchange
	asset  string
	symbol string
	solver *pricesolver.SimplePriceSolver

	// prices maps an hour start (unix ms) to the USDT price, 0 = fetched but unknown
	prices map[int64]fixedpoint.Value

	// fetchedFrom and fetchedTo are the hours a successful fetch covered; zero = none
	fetchedFrom, fetchedTo time.Time

	// misses counts the rows priced 0
	misses int

	now func() time.Time
}

func newHourlyOpenPricer(ex types.Exchange, markets types.MarketMap, asset string) *hourlyOpenPricer {
	p := &hourlyOpenPricer{
		ex:     ex,
		asset:  asset,
		solver: pricesolver.NewSimplePriceResolver(markets),
		prices: make(map[int64]fixedpoint.Value),
		now:    time.Now,
	}

	if m, ok := markets.FindPair(asset, interestPriceQuote); ok {
		p.symbol = m.Symbol
	}

	return p
}

func (p *hourlyOpenPricer) priceAt(ctx context.Context, t time.Time) (fixedpoint.Value, error) {
	if p.asset == interestPriceQuote {
		return fixedpoint.One, nil
	}

	// no USDT market: the price stays unknown
	if p.symbol == "" {
		p.misses++
		return fixedpoint.Zero, nil
	}

	hour := t.UTC().Truncate(time.Hour)
	key := hour.UnixMilli()
	price, ok := p.prices[key]
	recent := p.now().Sub(hour) < interestPriceRecentHours*time.Hour
	if !ok && p.fetched(hour) {
		// a fetch covered it after it was published: the hour has no kline
		p.prices[key] = fixedpoint.Zero
		price, ok = fixedpoint.Zero, true
	}

	if !ok {
		if err := p.fetch(ctx, hour); err != nil {
			return fixedpoint.Zero, fmt.Errorf("unable to query %s 1h klines for pricing margin interests: %w", p.symbol, err)
		}

		price, ok = p.prices[key]
		if !ok {
			if recent {
				return fixedpoint.Zero, fmt.Errorf("%s 1h kline at %s is not published yet", p.symbol, hour.Format(time.RFC3339))
			}

			// the market has no kline for this hour, e.g. it was listed later
			p.prices[key] = fixedpoint.Zero
			price = fixedpoint.Zero
		}
	}

	if price.IsZero() {
		p.misses++
	}

	return price, nil
}

// fetch prices every hour from hour until now. The klines received before an
// error are kept.
func (p *hourlyOpenPricer) fetch(ctx context.Context, hour time.Time) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	until := p.now()
	query := &batch.KLineBatchQuery{Exchange: p.ex, MaxRetries: interestPriceMaxRetries}
	klineC, errC := query.Query(ctx, p.symbol, types.Interval1h, hour, until)
	for k := range klineC {
		p.solver.Update(p.symbol, k.Open)
		if price, ok := p.solver.ResolvePrice(p.asset, interestPriceQuote); ok {
			p.prices[k.StartTime.Time().UTC().Truncate(time.Hour).UnixMilli()] = price
		}
	}

	if err := <-errC; err != nil {
		return err
	}

	// [hour, until] is contiguous with an earlier range, which ended at an earlier now
	if p.fetchedFrom.IsZero() || hour.Before(p.fetchedFrom) {
		p.fetchedFrom = hour
	}
	p.fetchedTo = until
	return nil
}

// fetched reports whether a successful fetch covered hour when it was old
// enough to be published. A recent hour may have been published since.
func (p *hourlyOpenPricer) fetched(hour time.Time) bool {
	return !p.fetchedFrom.IsZero() && !hour.Before(p.fetchedFrom) &&
		!hour.Add(interestPriceRecentHours*time.Hour).After(p.fetchedTo)
}

func (p *hourlyOpenPricer) logMisses() {
	if p.misses > 0 {
		logrus.Warnf("%d %s margin interest rows have no price at charge time", p.misses, p.asset)
	}
}
