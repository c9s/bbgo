package binancecsv

import (
	"fmt"
	"slices"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// Dataset names, matching the path segment on data.binance.vision.
const (
	DatasetAggTrades          = "aggTrades"
	DatasetTrades             = "trades"
	DatasetKLines             = "klines"
	DatasetIndexPriceKLines   = "indexPriceKlines"
	DatasetMarkPriceKLines    = "markPriceKlines"
	DatasetPremiumIndexKLines = "premiumIndexKlines"
	DatasetBookTicker         = "bookTicker"
	DatasetBookDepth          = "bookDepth"
	DatasetMetrics            = "metrics"
)

// Dataset describes one data.binance.vision product: where its files live, what
// it decodes into, and what it does not cover.
type Dataset struct {
	// Name is the path segment and the configuration value.
	Name string

	// Markets lists the archive trees that publish this dataset.
	Markets []Market

	// NeedsInterval is true for the kline families, whose files sit in a
	// per-interval subdirectory.
	NeedsInterval bool

	// Channels are the stream channels this dataset can satisfy.
	Channels []types.Channel

	// CoverageStart and CoverageEnd, when non-zero, bound publication. They
	// exist so a request outside the window fails with a message naming the
	// real range instead of downloading a wall of 404s.
	CoverageStart time.Time
	CoverageEnd   time.Time

	// NewDecoder builds the per-record decoder for this dataset.
	NewDecoder func(cfg Config) RecordDecoder
}

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// datasets is the registry. It lists only products that actually exist: a
// listing of data/futures/um/daily/ yields aggTrades, bookDepth, bookTicker,
// indexPriceKlines, klines, markPriceKlines, metrics, premiumIndexKlines and
// trades — there is no liquidationSnapshot and no depth diff.
var datasets = map[string]Dataset{
	DatasetAggTrades: {
		Name:       DatasetAggTrades,
		Markets:    []Market{MarketSpot, MarketUSDMFutures, MarketCOINMFutures},
		Channels:   []types.Channel{types.AggTradeChannel, types.MarketTradeChannel},
		NewDecoder: func(cfg Config) RecordDecoder { return newAggTradeDecoder(cfg) },
	},
	DatasetTrades: {
		Name:       DatasetTrades,
		Markets:    []Market{MarketSpot, MarketUSDMFutures, MarketCOINMFutures},
		Channels:   []types.Channel{types.MarketTradeChannel},
		NewDecoder: func(cfg Config) RecordDecoder { return newTradeDecoder(cfg) },
	},
	DatasetKLines: {
		Name:          DatasetKLines,
		Markets:       []Market{MarketSpot, MarketUSDMFutures, MarketCOINMFutures},
		NeedsInterval: true,
		Channels:      []types.Channel{types.KLineChannel},
		NewDecoder:    func(cfg Config) RecordDecoder { return newKLineDecoder(cfg) },
	},
	DatasetIndexPriceKLines: {
		Name:          DatasetIndexPriceKLines,
		Markets:       []Market{MarketUSDMFutures, MarketCOINMFutures},
		NeedsInterval: true,
		Channels:      []types.Channel{types.IndexPriceKLineChannel},
		NewDecoder:    func(cfg Config) RecordDecoder { return newKLineDecoder(cfg) },
	},
	DatasetMarkPriceKLines: {
		Name:          DatasetMarkPriceKLines,
		Markets:       []Market{MarketUSDMFutures, MarketCOINMFutures},
		NeedsInterval: true,
		Channels:      []types.Channel{types.MarkPriceChannel},
		NewDecoder:    func(cfg Config) RecordDecoder { return newKLineDecoder(cfg) },
	},
	DatasetPremiumIndexKLines: {
		Name:          DatasetPremiumIndexKLines,
		Markets:       []Market{MarketUSDMFutures, MarketCOINMFutures},
		NeedsInterval: true,
		Channels:      []types.Channel{types.KLineChannel},
		NewDecoder:    func(cfg Config) RecordDecoder { return newKLineDecoder(cfg) },
	},
	DatasetBookTicker: {
		Name:     DatasetBookTicker,
		Markets:  []Market{MarketUSDMFutures, MarketCOINMFutures},
		Channels: []types.Channel{types.BookTickerChannel},
		// Verified against the S3 listing: the first file is 2023-05-16 and the
		// last is 2024-03-30. Binance stopped publishing it.
		CoverageStart: mustDate("2023-05-16"),
		CoverageEnd:   mustDate("2024-03-31"),
		NewDecoder:    func(cfg Config) RecordDecoder { return newBookTickerDecoder(cfg) },
	},
	DatasetBookDepth: {
		Name:    DatasetBookDepth,
		Markets: []Market{MarketUSDMFutures, MarketCOINMFutures},
		// Deliberately not BookChannel: this dataset carries no price levels.
		Channels:   nil,
		NewDecoder: func(cfg Config) RecordDecoder { return newBookDepthDecoder(cfg) },
	},
	DatasetMetrics: {
		Name:       DatasetMetrics,
		Markets:    []Market{MarketUSDMFutures, MarketCOINMFutures},
		Channels:   nil,
		NewDecoder: func(cfg Config) RecordDecoder { return newMetricsDecoder(cfg) },
	},
}

// LookupDataset returns the dataset descriptor by name.
func LookupDataset(name string) (Dataset, error) {
	ds, ok := datasets[name]
	if !ok {
		return Dataset{}, fmt.Errorf("binancecsv: unknown dataset %q, known datasets are %v",
			name, DatasetNames())
	}
	return ds, nil
}

// DatasetNames returns every known dataset name, sorted.
func DatasetNames() []string {
	out := make([]string, 0, len(datasets))
	for name := range datasets {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// supportsMarket reports whether the dataset is published for m.
func (d Dataset) supportsMarket(m Market) bool { return slices.Contains(d.Markets, m) }

// coversRange reports whether [since, until) lies inside the dataset's
// publication window.
func (d Dataset) coversRange(since, until time.Time) error {
	if !d.CoverageStart.IsZero() && since.Before(d.CoverageStart) {
		return fmt.Errorf("binancecsv: dataset %s starts at %s, requested from %s",
			d.Name, d.CoverageStart.Format(time.DateOnly), since.UTC().Format(time.DateOnly))
	}
	if !d.CoverageEnd.IsZero() && until.After(d.CoverageEnd) {
		return fmt.Errorf("binancecsv: dataset %s was discontinued after %s, requested until %s",
			d.Name, d.CoverageEnd.Format(time.DateOnly), until.UTC().Format(time.DateOnly))
	}
	return nil
}
