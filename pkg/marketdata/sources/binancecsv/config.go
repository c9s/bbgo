package binancecsv

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/marketdata/archive"
	"github.com/c9s/bbgo/pkg/types"
)

// Config configures a binancecsv Source.
type Config struct {
	// Market selects the archive tree. Required.
	Market Market `json:"market" yaml:"market"`

	// Period selects daily or monthly archives. Defaults to daily.
	Period Period `json:"period,omitempty" yaml:"period,omitempty"`

	// Datasets lists the products to read, for example aggTrades and klines.
	// Required.
	Datasets []string `json:"datasets" yaml:"datasets"`

	// Symbols restricts the source. When empty it serves whatever the request
	// asks for.
	Symbols []string `json:"symbols,omitempty" yaml:"symbols,omitempty"`

	// Intervals lists the kline intervals to read when a kline dataset is
	// configured. When empty the intervals come from the request's
	// subscriptions.
	Intervals []types.Interval `json:"intervals,omitempty" yaml:"intervals,omitempty"`

	// CacheDir is the archive cache root. Required unless Cache is supplied.
	CacheDir string `json:"cacheDir,omitempty" yaml:"cacheDir,omitempty"`

	// Cache overrides the default cache, for tests or for sharing a rate
	// limiter across sources.
	Cache *archive.HTTPCache `json:"-" yaml:"-"`

	// AllowMissingDays downgrades an archive that the publisher does not have
	// from an error to a warning.
	//
	// It defaults to false on purpose. The previous implementation treated
	// every failure as end-of-data, so a 404 in the middle of a range produced
	// a short dataset that looked complete; tolerating a gap should be an
	// explicit choice.
	AllowMissingDays bool `json:"allowMissingDays,omitempty" yaml:"allowMissingDays,omitempty"`

	// SynthesizeBookFromBookTicker makes the bookTicker dataset additionally
	// emit a one-level book snapshot per record, flagged FlagSynthetic and
	// FlagPartialDepth.
	//
	// Off by default. It is a development aid for code that needs something
	// book-shaped before real L2 is available, not a substitute for L2: a book
	// with one level per side has no depth, and the dataset itself stops at
	// 2024-03-30.
	SynthesizeBookFromBookTicker bool `json:"synthesizeBookFromBookTicker,omitempty" yaml:"synthesizeBookFromBookTicker,omitempty"`

	// Name overrides the source name used in logs and merge diagnostics.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

func (c *Config) applyDefaults() {
	if c.Period == "" {
		c.Period = PeriodDaily
	}
}

func (c *Config) validate() error {
	if err := c.Market.validate(); err != nil {
		return err
	}
	if err := c.Period.validate(); err != nil {
		return err
	}
	if len(c.Datasets) == 0 {
		return fmt.Errorf("binancecsv: at least one dataset is required, known datasets are %v",
			DatasetNames())
	}

	for _, name := range c.Datasets {
		ds, err := LookupDataset(name)
		if err != nil {
			return err
		}
		if !ds.supportsMarket(c.Market) {
			return fmt.Errorf("binancecsv: dataset %s is not published for market %s, only for %v",
				name, c.Market, ds.Markets)
		}
	}

	if c.Cache == nil && c.CacheDir == "" {
		return fmt.Errorf("binancecsv: cacheDir is required")
	}

	return nil
}
