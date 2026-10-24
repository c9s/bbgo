package amberdata

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/marketdata/sources/amberdata/amberdataapi"
)

// Query is what the Source asks a Client for: one endpoint, one instrument, one
// time window.
//
// It sits here rather than in amberdataapi because the request builders there
// are generated per endpoint with their own fields; this is the small common
// shape the paginator drives them through.
type Query struct {
	// Exchange is required, and only one is accepted per request.
	Exchange string

	// Instrument is the venue's own symbol.
	Instrument string

	// Since is inclusive, Until exclusive.
	Since time.Time
	Until time.Time

	// MaxLevel caps order book depth. Zero leaves it to the server.
	MaxLevel int

	// TimeInterval selects the OHLCV bucket: minutes, hours or days.
	TimeInterval string
}

// Client is the surface the Source needs.
//
// It is an interface rather than the concrete REST client because several
// response semantics are unverifiable without a paid key — see doc.go — and this
// keeps those corrections, and the fake used in tests, to one place.
//
// Every method takes a cursor URL that, when non-empty, supersedes the query:
// AmberData's pagination cursor is an opaque absolute URL, so continuing a result
// set is a different request rather than a different parameter.
type Client interface {
	GetTrades(ctx context.Context, q Query, next string) (amberdataapi.Page[amberdataapi.Trade], error)
	GetOrderBookSnapshots(ctx context.Context, q Query, next string) (amberdataapi.Page[amberdataapi.Book], error)
	GetOrderBookEvents(ctx context.Context, q Query, next string) (amberdataapi.Page[amberdataapi.Book], error)
	GetOHLCV(ctx context.Context, q Query, next string) (amberdataapi.Page[amberdataapi.OHLCV], error)
	GetFundingRates(ctx context.Context, q Query, next string) (amberdataapi.Page[amberdataapi.FundingRate], error)
	GetOpenInterest(ctx context.Context, q Query, next string) (amberdataapi.Page[amberdataapi.OpenInterest], error)
	GetLiquidations(ctx context.Context, q Query, next string) (amberdataapi.Page[amberdataapi.Liquidation], error)
	GetLongShortRatio(ctx context.Context, q Query, next string) (amberdataapi.Page[amberdataapi.LongShortRatio], error)
	GetTickers(ctx context.Context, q Query, next string) (amberdataapi.Page[amberdataapi.Ticker], error)
}

// CoverageClient reports the range each instrument has data for. It is separate
// from Client because the Source does not need it to replay — it is for planning
// a backfill, and for answering "does this instrument have 2019 data".
type CoverageClient interface {
	GetCoverage(ctx context.Context, feature, exchange string) ([]amberdataapi.InstrumentCoverage, error)
	GetReference(ctx context.Context, exchange, instrument string) ([]amberdataapi.InstrumentReference, error)
}

// restClient adapts the generated request builders to Client.
type restClient struct {
	rest  *amberdataapi.RestClient
	class amberdataapi.AssetClass
}

var (
	_ Client         = (*restClient)(nil)
	_ CoverageClient = (*restClient)(nil)
)

// NewClient returns a Client backed by the REST API.
func NewClient(apiKey string, class amberdataapi.AssetClass) Client {
	rest := amberdataapi.NewRestClient()
	rest.Auth(apiKey)
	return &restClient{rest: rest, class: class}
}

// NewClientWithRest wraps an existing REST client, for tests that supply a
// mocked transport.
func NewClientWithRest(rest *amberdataapi.RestClient, class amberdataapi.AssetClass) Client {
	return &restClient{rest: rest, class: class}
}

// NewCoverageClient returns a CoverageClient backed by the REST API.
func NewCoverageClient(apiKey string, class amberdataapi.AssetClass) CoverageClient {
	rest := amberdataapi.NewRestClient()
	rest.Auth(apiKey)
	return &restClient{rest: rest, class: class}
}

func (c *restClient) GetTrades(
	ctx context.Context, q Query, next string,
) (amberdataapi.Page[amberdataapi.Trade], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.Trade](ctx, c.rest, next)
	}

	req := c.rest.NewGetTradesRequest(c.class).
		Instrument(q.Instrument).
		Exchange(q.Exchange)
	applyWindow(q, req.StartDate, req.EndDate)

	return amberdataapi.DoPage(ctx, req.Do)
}

func (c *restClient) GetOrderBookSnapshots(
	ctx context.Context, q Query, next string,
) (amberdataapi.Page[amberdataapi.Book], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.Book](ctx, c.rest, next)
	}

	req := c.rest.NewGetOrderBookSnapshotsRequest(c.class).
		Instrument(q.Instrument).
		Exchange(q.Exchange)
	applyWindow(q, req.StartDate, req.EndDate)
	if q.MaxLevel > 0 {
		req.MaxLevel(q.MaxLevel)
	}

	return amberdataapi.DoPage(ctx, req.Do)
}

func (c *restClient) GetOrderBookEvents(
	ctx context.Context, q Query, next string,
) (amberdataapi.Page[amberdataapi.Book], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.Book](ctx, c.rest, next)
	}

	req := c.rest.NewGetOrderBookEventsRequest(c.class).
		Instrument(q.Instrument).
		Exchange(q.Exchange)
	applyWindow(q, req.StartDate, req.EndDate)
	if q.MaxLevel > 0 {
		req.MaxLevel(q.MaxLevel)
	}

	return amberdataapi.DoPage(ctx, req.Do)
}

func (c *restClient) GetOHLCV(
	ctx context.Context, q Query, next string,
) (amberdataapi.Page[amberdataapi.OHLCV], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.OHLCV](ctx, c.rest, next)
	}

	req := c.rest.NewGetOHLCVRequest(c.class).
		Instrument(q.Instrument).
		Exchange(q.Exchange)
	applyWindow(q, req.StartDate, req.EndDate)
	if q.TimeInterval != "" {
		req.TimeInterval(q.TimeInterval)
	}

	return amberdataapi.DoPage(ctx, req.Do)
}

func (c *restClient) GetFundingRates(
	ctx context.Context, q Query, next string,
) (amberdataapi.Page[amberdataapi.FundingRate], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.FundingRate](ctx, c.rest, next)
	}

	req := c.rest.NewGetFundingRatesRequest(c.class).
		Instrument(q.Instrument).
		Exchange(q.Exchange)
	applyWindow(q, req.StartDate, req.EndDate)

	return amberdataapi.DoPage(ctx, req.Do)
}

func (c *restClient) GetOpenInterest(
	ctx context.Context, q Query, next string,
) (amberdataapi.Page[amberdataapi.OpenInterest], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.OpenInterest](ctx, c.rest, next)
	}

	req := c.rest.NewGetOpenInterestRequest(c.class).
		Instrument(q.Instrument).
		Exchange(q.Exchange)
	applyWindow(q, req.StartDate, req.EndDate)

	return amberdataapi.DoPage(ctx, req.Do)
}

func (c *restClient) GetLiquidations(
	ctx context.Context, q Query, next string,
) (amberdataapi.Page[amberdataapi.Liquidation], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.Liquidation](ctx, c.rest, next)
	}

	req := c.rest.NewGetLiquidationsRequest(c.class).
		Instrument(q.Instrument).
		Exchange(q.Exchange)
	applyWindow(q, req.StartDate, req.EndDate)

	return amberdataapi.DoPage(ctx, req.Do)
}

func (c *restClient) GetLongShortRatio(
	ctx context.Context, q Query, next string,
) (amberdataapi.Page[amberdataapi.LongShortRatio], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.LongShortRatio](ctx, c.rest, next)
	}

	req := c.rest.NewGetLongShortRatioRequest(c.class).
		Instrument(q.Instrument).
		Exchange(q.Exchange)
	applyWindow(q, req.StartDate, req.EndDate)

	return amberdataapi.DoPage(ctx, req.Do)
}

func (c *restClient) GetTickers(
	ctx context.Context, q Query, next string,
) (amberdataapi.Page[amberdataapi.Ticker], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.Ticker](ctx, c.rest, next)
	}

	req := c.rest.NewGetTickersRequest(c.class).
		Instrument(q.Instrument).
		Exchange(q.Exchange)
	applyWindow(q, req.StartDate, req.EndDate)

	return amberdataapi.DoPage(ctx, req.Do)
}

func (c *restClient) GetCoverage(
	ctx context.Context, feature, exchange string,
) ([]amberdataapi.InstrumentCoverage, error) {
	req := c.rest.NewGetCoverageRequest(c.class, feature)
	if exchange != "" {
		req.Exchange(exchange)
	}

	page, err := amberdataapi.DoPage(ctx, req.Do)
	return page.Data, err
}

func (c *restClient) GetReference(
	ctx context.Context, exchange, instrument string,
) ([]amberdataapi.InstrumentReference, error) {
	req := c.rest.NewGetExchangesReferenceRequest(c.class)
	if exchange != "" {
		req.Exchange(exchange)
	}
	if instrument != "" {
		req.Instrument(instrument)
	}

	page, err := amberdataapi.DoPage(ctx, req.Do)
	return page.Data, err
}

// applyWindow sets the date bounds only when they are non-zero: omitting both
// makes the API return the previous 24 hours, which is the right behaviour for a
// caller that did not ask for a range.
func applyWindow[T any](q Query, setSince, setUntil func(time.Time) T) {
	if !q.Since.IsZero() {
		setSince(q.Since)
	}
	if !q.Until.IsZero() {
		setUntil(q.Until)
	}
}

// ohlcvInterval maps a kline interval to the API's timeInterval bucket. The API
// exposes only three granularities, so anything finer than a minute or coarser
// than a day has no equivalent.
func ohlcvInterval(d time.Duration) (string, bool) {
	switch {
	case d < time.Minute:
		return "", false
	case d < time.Hour:
		return "minutes", true
	case d < 24*time.Hour:
		return "hours", true
	case d == 24*time.Hour:
		return "days", true
	default:
		return "", false
	}
}
