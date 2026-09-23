package binancecsv

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/archive"
	"github.com/c9s/bbgo/pkg/types"
)

// Source reads Binance's published CSV archives as market data events.
type Source struct {
	cfg   Config
	cache *archive.HTTPCache
}

var _ marketdata.Source = (*Source)(nil)

// New builds a Source from cfg.
func New(cfg Config) (*Source, error) {
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cache := cfg.Cache
	if cache == nil {
		cache = &archive.HTTPCache{Dir: cfg.CacheDir}
	}

	return &Source{cfg: cfg, cache: cache}, nil
}

func (s *Source) Name() string {
	if s.cfg.Name != "" {
		return s.cfg.Name
	}
	return fmt.Sprintf("binancecsv/%s", s.cfg.Market)
}

// Capabilities reports what this source serves.
//
// types.BookChannel is deliberately absent: data.binance.vision publishes no
// L2 data. See the package comment.
func (s *Source) Capabilities() marketdata.Capability {
	cap := marketdata.Capability{
		Exchanges:  []types.ExchangeName{types.ExchangeBinance},
		Symbols:    s.cfg.Symbols,
		Intervals:  s.cfg.Intervals,
		HasHistory: true,
	}

	for _, name := range s.cfg.Datasets {
		ds, err := LookupDataset(name)
		if err != nil {
			continue
		}
		for _, ch := range ds.Channels {
			if !slices.Contains(cap.Channels, ch) {
				cap.Channels = append(cap.Channels, ch)
			}
		}
		if !ds.CoverageStart.IsZero() &&
			(cap.CoverageStart.IsZero() || ds.CoverageStart.Before(cap.CoverageStart)) {
			cap.CoverageStart = ds.CoverageStart
		}
		if !ds.CoverageEnd.IsZero() &&
			(cap.CoverageEnd.IsZero() || ds.CoverageEnd.After(cap.CoverageEnd)) {
			cap.CoverageEnd = ds.CoverageEnd
		}

		if s.cfg.SynthesizeBookFromBookTicker && ds.Name == DatasetBookTicker {
			// A synthesized one-level book is not real L2, so this still does
			// not advertise BookChannel; the events are emitted for anyone
			// already consuming the stream, flagged FlagSynthetic.
			continue
		}
	}

	return cap
}

// Open resolves the request into a list of archives and returns a cursor that
// streams them in order.
func (s *Source) Open(ctx context.Context, req marketdata.Request) (marketdata.Cursor, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	plan, err := s.plan(req)
	if err != nil {
		return nil, err
	}

	if len(plan) == 0 {
		return nil, &marketdata.UnsupportedError{
			Source: s.Name(),
			Reason: "no configured dataset matches the requested subscriptions",
		}
	}

	// One cursor per series, merged: see the seriesCursor doc comment for why a
	// flat date-ordered list of every archive would not be monotonic.
	series := groupIntoSeries(plan)
	if len(series) == 1 {
		return newSeriesCursor(ctx, s, series[0].tasks, req), nil
	}

	cursors := make([]marketdata.NamedCursor, len(series))
	for i, sr := range series {
		cursors[i] = marketdata.NamedCursor{
			Name:   s.Name() + "/" + sr.key,
			Cursor: newSeriesCursor(ctx, s, sr.tasks, req),
		}
	}

	return marketdata.Merge(cursors), nil
}

// series is one dataset, symbol and interval, with its archives in date order.
type series struct {
	key   string
	tasks []fileTask
}

// groupIntoSeries splits a plan into monotonic series, keeping a deterministic
// order so a merge over them is reproducible.
func groupIntoSeries(plan []fileTask) []series {
	index := map[string]int{}
	var out []series

	for _, task := range plan {
		key := fmt.Sprintf("%s/%s", task.meta.Dataset, task.meta.Symbol)
		if len(task.meta.Interval) > 0 {
			key += "/" + string(task.meta.Interval)
		}

		if i, ok := index[key]; ok {
			out[i].tasks = append(out[i].tasks, task)
			continue
		}

		index[key] = len(out)
		out = append(out, series{key: key, tasks: []fileTask{task}})
	}

	for i := range out {
		slices.SortStableFunc(out[i].tasks, func(a, b fileTask) int {
			return a.ref.Date.Compare(b.ref.Date)
		})
	}

	return out
}

// fileTask is one archive to read, with the metadata its records inherit.
type fileTask struct {
	ref     FileRef
	dataset Dataset
	decoder RecordDecoder
	meta    RecordMeta
}

// plan turns a request into the ordered list of archives to read.
//
// Files are ordered by the time they cover so that the cursor, which reads them
// one at a time, produces a non-decreasing stream without buffering more than
// one file's batch.
func (s *Source) plan(req marketdata.Request) ([]fileTask, error) {
	if err := s.rejectUnsupported(req); err != nil {
		return nil, err
	}

	var tasks []fileTask

	for _, name := range s.cfg.Datasets {
		ds, err := LookupDataset(name)
		if err != nil {
			return nil, err
		}

		symbols, intervals := s.resolve(ds, req)
		if len(symbols) == 0 {
			// Nothing in this request selects this dataset; another source, or
			// another dataset of this source, covers those subscriptions.
			continue
		}
		if ds.NeedsInterval && len(intervals) == 0 {
			return nil, fmt.Errorf(
				"binancecsv: dataset %s needs an interval; set intervals in the config or subscribe to a kline channel",
				ds.Name)
		}
		// Only checked for datasets this request actually reads, so a config
		// listing a discontinued dataset does not break unrelated requests.
		if err := ds.coversRange(req.Since, req.Until); err != nil {
			return nil, err
		}

		if !ds.NeedsInterval {
			intervals = []types.Interval{""}
		}

		decoder := ds.NewDecoder(s.cfg)

		for _, symbol := range symbols {
			for _, interval := range intervals {
				for _, ref := range enumerateFiles(
					s.cfg.Market, s.cfg.Period, ds.Name, symbol, interval, req.Since, req.Until,
				) {
					tasks = append(tasks, fileTask{
						ref:     ref,
						dataset: ds,
						decoder: decoder,
						meta: RecordMeta{
							Exchange: types.ExchangeBinance,
							Market:   s.cfg.Market,
							Symbol:   symbol,
							Dataset:  ds.Name,
							Interval: interval,
							File:     ref.FileName(),
						},
					})
				}
			}
		}
	}

	return tasks, nil
}

// rejectUnsupported fails a request this source can contribute nothing to.
//
// It does not reject a request that merely contains subscriptions this source
// cannot serve: in a merge each source serves its own share, and whether the
// request as a whole is covered is checked by marketdata.CheckCoverage, which
// can see every source. What this does do is explain the one case people will
// hit and misdiagnose — asking Binance's archives for an order book.
func (s *Source) rejectUnsupported(req marketdata.Request) error {
	capabilities := s.Capabilities()

	if len(capabilities.Covered(req)) > 0 {
		return nil
	}

	for _, sub := range req.Subscriptions {
		if sub.Channel != types.BookChannel {
			continue
		}

		// This is a property of the data source, not of the configuration, so
		// no amount of reconfiguring will fix it. Say so.
		return &marketdata.UnsupportedError{
			Source:  s.Name(),
			Channel: sub.Channel,
			Symbol:  sub.Symbol,
			Reason: "data.binance.vision publishes no L2 order book data: " +
				"bookTicker is L1 only and was discontinued after 2024-03-30, and " +
				"bookDepth is percentage-band notional with no price levels. " +
				"Use a vendor feed or a local recording for L2",
		}
	}

	return &marketdata.UnsupportedError{
		Source: s.Name(),
		Reason: fmt.Sprintf("configured datasets %v serve channels %v, none of which was requested",
			s.cfg.Datasets, capabilities.Channels),
	}
}

// resolve picks the symbols and intervals for one dataset, preferring the
// request's subscriptions and falling back to the configuration.
func (s *Source) resolve(ds Dataset, req marketdata.Request) ([]string, []types.Interval) {
	var symbols []string
	var intervals []types.Interval

	for _, sub := range req.Subscriptions {
		if !slices.Contains(ds.Channels, sub.Channel) {
			continue
		}
		if len(s.cfg.Symbols) > 0 && !slices.Contains(s.cfg.Symbols, sub.Symbol) {
			continue
		}
		if !slices.Contains(symbols, sub.Symbol) {
			symbols = append(symbols, sub.Symbol)
		}
		if ds.NeedsInterval && len(sub.Options.Interval) > 0 &&
			!slices.Contains(intervals, sub.Options.Interval) {
			intervals = append(intervals, sub.Options.Interval)
		}
	}

	// Datasets with no stream channel — bookDepth and metrics — are never
	// selected by a subscription, so they fall back to the configured symbols.
	if len(ds.Channels) == 0 {
		symbols = slices.Clone(s.cfg.Symbols)
		if len(symbols) == 0 {
			symbols = req.Symbols()
		}
	}

	if len(s.cfg.Intervals) > 0 {
		intervals = slices.Clone(s.cfg.Intervals)
	}

	return symbols, intervals
}

// FetchAll downloads every archive a request needs without decoding them. It
// backs the download CLI, where warming the cache is the whole point.
func (s *Source) FetchAll(ctx context.Context, req marketdata.Request) ([]string, error) {
	plan, err := s.plan(req)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, task := range plan {
		path, err := s.cache.Fetch(ctx, task.ref.URL())
		if err != nil {
			if isMissing(err) && s.cfg.AllowMissingDays {
				continue
			}
			return paths, err
		}
		paths = append(paths, path)
	}

	return paths, nil
}

// Files lists the archives a request resolves to, for dry runs.
func (s *Source) Files(req marketdata.Request) ([]FileRef, error) {
	plan, err := s.plan(req)
	if err != nil {
		return nil, err
	}

	refs := make([]FileRef, len(plan))
	for i, task := range plan {
		refs[i] = task.ref
	}
	return refs, nil
}

// periodEnd returns the exclusive end of the period a ref covers, used to skip
// archives that fall entirely outside the requested range.
func (s *Source) periodEnd(ref FileRef) time.Time { return s.cfg.Period.next(ref.Date) }
