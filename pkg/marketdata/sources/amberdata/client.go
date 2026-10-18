package amberdata

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/marketdata/sources/amberdata/amberdataapi"
)

// Client is the surface the Source needs.
//
// It is an interface rather than a concrete client because several response
// semantics are unverifiable without a paid key — see the package comment — and
// this keeps those corrections, and the fake used in tests, to one place.
type Client interface {
	// GetTrades returns one page of trades. next, when non-empty, is a cursor
	// URL from a previous page and takes precedence over the query.
	GetTrades(ctx context.Context, q amberdataapi.Query, next string) (amberdataapi.Page[amberdataapi.Trade], error)

	// GetOrderBookSnapshots returns one page of complete book states.
	GetOrderBookSnapshots(ctx context.Context, q amberdataapi.Query, next string) (amberdataapi.Page[amberdataapi.Book], error)

	// GetOrderBookEvents returns one page of incremental book changes, in which
	// a level with volume zero means removal.
	GetOrderBookEvents(ctx context.Context, q amberdataapi.Query, next string) (amberdataapi.Page[amberdataapi.Book], error)

	// GetOHLCV returns one page of candles.
	GetOHLCV(ctx context.Context, q amberdataapi.Query, next string) (amberdataapi.Page[amberdataapi.OHLCV], error)
}

// restClient adapts amberdataapi.RestClient to Client, folding the cursor and
// query paths into one call so the paginator does not care which it is using.
type restClient struct {
	rest  *amberdataapi.RestClient
	class amberdataapi.AssetClass
}

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

func (c *restClient) GetTrades(
	ctx context.Context, q amberdataapi.Query, next string,
) (amberdataapi.Page[amberdataapi.Trade], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.Trade](ctx, c.rest, next)
	}
	return c.rest.GetTrades(ctx, c.class, q)
}

func (c *restClient) GetOrderBookSnapshots(
	ctx context.Context, q amberdataapi.Query, next string,
) (amberdataapi.Page[amberdataapi.Book], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.Book](ctx, c.rest, next)
	}
	return c.rest.GetOrderBookSnapshots(ctx, c.class, q)
}

func (c *restClient) GetOrderBookEvents(
	ctx context.Context, q amberdataapi.Query, next string,
) (amberdataapi.Page[amberdataapi.Book], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.Book](ctx, c.rest, next)
	}
	return c.rest.GetOrderBookEvents(ctx, c.class, q)
}

func (c *restClient) GetOHLCV(
	ctx context.Context, q amberdataapi.Query, next string,
) (amberdataapi.Page[amberdataapi.OHLCV], error) {
	if next != "" {
		return amberdataapi.GetCursor[amberdataapi.OHLCV](ctx, c.rest, next)
	}
	return c.rest.GetOHLCV(ctx, c.class, q)
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
