// Package binancecsv reads the historical CSV archives Binance publishes at
// data.binance.vision and turns them into market data events.
//
// # What this source can and cannot serve
//
// data.binance.vision publishes no L2 order book data at all. There is no
// incremental depth diff dataset and no price-level snapshot dataset. The two
// datasets whose names suggest otherwise do not help:
//
//   - bookTicker is L1 only — one best bid and one best ask — and it was
//     discontinued: the last file is 2024-03-30, with coverage starting
//     2023-05-16.
//   - bookDepth is aggregate notional within percentage bands of mid, sampled
//     about once a minute. It has no prices, so no book can be reconstructed
//     from it.
//
// Consequently this source does not declare types.BookChannel and rejects a
// request for it rather than returning an empty cursor. Real L2 for Binance
// futures has to come from a vendor such as AmberData, or from a local
// recording of the live websocket feed.
package binancecsv

import (
	"fmt"
	"strings"
	"time"
)

// Market selects which of the three archive trees to read from.
type Market string

const (
	// MarketSpot is data/spot.
	MarketSpot Market = "spot"

	// MarketUSDMFutures is data/futures/um: USDⓈ-M contracts, symbols like
	// BTCUSDT.
	MarketUSDMFutures Market = "um"

	// MarketCOINMFutures is data/futures/cm: COIN-M contracts, symbols like
	// BTCUSD_PERP.
	MarketCOINMFutures Market = "cm"
)

// ParseMarket accepts the canonical names plus the aliases people actually
// type in configuration files.
func ParseMarket(s string) (Market, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "spot":
		return MarketSpot, nil
	case "um", "usdm", "futures", "futures/um":
		return MarketUSDMFutures, nil
	case "cm", "coinm", "futures/cm":
		return MarketCOINMFutures, nil
	default:
		return "", fmt.Errorf("binancecsv: unknown market %q, want spot, um or cm", s)
	}
}

// IsFutures reports whether the market is one of the futures trees.
func (m Market) IsFutures() bool {
	return m == MarketUSDMFutures || m == MarketCOINMFutures
}

// pathPrefix is the market's segment of a data.binance.vision URL.
func (m Market) pathPrefix() string {
	if m.IsFutures() {
		return "futures/" + string(m)
	}
	return string(m)
}

func (m Market) validate() error {
	switch m {
	case MarketSpot, MarketUSDMFutures, MarketCOINMFutures:
		return nil
	default:
		return fmt.Errorf("binancecsv: unknown market %q", m)
	}
}

// Period selects daily or monthly archives. Monthly files cover a whole month
// in one download and are the right choice for a long backfill; daily files are
// the only option for the current month.
type Period string

const (
	PeriodDaily   Period = "daily"
	PeriodMonthly Period = "monthly"
)

// dateFormat is the date component of an archive file name.
func (p Period) dateFormat() string {
	if p == PeriodMonthly {
		return "2006-01"
	}
	return "2006-01-02"
}

// truncate returns the start of the period containing t, in UTC. Archives are
// published per UTC day or month.
func (p Period) truncate(t time.Time) time.Time {
	t = t.UTC()
	if p == PeriodMonthly {
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// next returns the start of the period after the one containing t.
func (p Period) next(t time.Time) time.Time {
	start := p.truncate(t)
	if p == PeriodMonthly {
		return start.AddDate(0, 1, 0)
	}
	return start.AddDate(0, 0, 1)
}

func (p Period) validate() error {
	switch p {
	case PeriodDaily, PeriodMonthly:
		return nil
	default:
		return fmt.Errorf("binancecsv: unknown period %q, want daily or monthly", p)
	}
}
