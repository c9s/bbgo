package binancecsv

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// BaseURL is the public archive host.
const BaseURL = "https://data.binance.vision"

// FileRef identifies one archive: everything needed to build its URL and to
// attribute the records it contains.
type FileRef struct {
	Market   Market
	Period   Period
	Dataset  string
	Symbol   string
	Interval types.Interval // kline families only
	Date     time.Time      // start of the day or month the archive covers
}

// FileName returns the archive file name, for example
// "BTCUSDT-aggTrades-2026-02-15.zip" or "BTCUSDT-1h-2026-02.zip".
//
// The kline families name the file after the interval rather than the dataset,
// which is why this is not a single format string.
func (f FileRef) FileName() string {
	date := f.Date.UTC().Format(f.Period.dateFormat())

	if len(f.Interval) > 0 {
		return fmt.Sprintf("%s-%s-%s.zip", f.Symbol, f.Interval, date)
	}
	return fmt.Sprintf("%s-%s-%s.zip", f.Symbol, f.Dataset, date)
}

// URLPath returns the archive's path on data.binance.vision, without the host.
func (f FileRef) URLPath() string {
	dir := fmt.Sprintf("data/%s/%s/%s/%s",
		f.Market.pathPrefix(), f.Period, f.Dataset, f.Symbol)

	if len(f.Interval) > 0 {
		dir += "/" + string(f.Interval)
	}

	return dir + "/" + f.FileName()
}

// URL returns the full archive URL.
func (f FileRef) URL() string { return BaseURL + "/" + f.URLPath() }

// ChecksumURL returns the URL of the sidecar published next to the archive.
func (f FileRef) ChecksumURL() string { return f.URL() + ".CHECKSUM" }

// enumerateFiles lists the archives covering [since, until) for one symbol.
//
// The range is half-open, so a request ending exactly at a period boundary does
// not pull the following archive.
func enumerateFiles(
	market Market, period Period, dataset, symbol string, interval types.Interval,
	since, until time.Time,
) []FileRef {
	var out []FileRef

	for cur := period.truncate(since); cur.Before(until); cur = period.next(cur) {
		out = append(out, FileRef{
			Market:   market,
			Period:   period,
			Dataset:  dataset,
			Symbol:   symbol,
			Interval: interval,
			Date:     cur,
		})
	}

	return out
}
