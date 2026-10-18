package amberdataapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AssetClass selects the /futures or /spot family of endpoints.
type AssetClass string

const (
	AssetClassFutures AssetClass = "futures"
	AssetClassSpot    AssetClass = "spot"
)

// Page is one page of results plus the cursor that continues it.
type Page[T any] struct {
	Data []T

	// Next is metadata.next verbatim: a complete absolute URL. Empty when the
	// result set is exhausted.
	Next string

	// ReturnedStart and ReturnedEnd are the range the server actually served,
	// which can be narrower than the one requested.
	ReturnedStart time.Time
	ReturnedEnd   time.Time
}

// Query is the set of parameters every historical endpoint shares.
type Query struct {
	// Exchange is required, and exactly one is allowed.
	Exchange string

	// Instrument is the venue's own symbol: BTCUSDT for Binance USDⓈ-M,
	// BTCUSD_PERP for COIN-M, btc_usd for spot.
	Instrument string

	// Since is inclusive and Until exclusive. When both are zero the API
	// returns the previous 24 hours.
	Since time.Time
	Until time.Time

	// MaxLevel caps order book depth. Zero leaves it to the server.
	MaxLevel int

	// TimeInterval selects the OHLCV bucket: minutes, hours or days.
	TimeInterval string
}

// values renders the query.
//
// timeFormat is always milliseconds and is deliberately not configurable: the
// server's default is `hr`, which emits "2024-06-04 16:23:12 414" — a
// space-separated millisecond field that is neither RFC3339 nor a number. Making
// it a parameter would only create a way to get it wrong.
//
// sortDirection is never sent. The API accepts it only when the window lies
// within the last 24 hours or no dates are given at all, and rejects it
// outright on a historical range; ascending is both the default and what a
// replay wants.
func (q Query) values() (url.Values, error) {
	if q.Exchange == "" {
		return nil, fmt.Errorf("amberdata: exchange is required and only one is allowed")
	}

	v := url.Values{}
	v.Set("exchange", q.Exchange)
	v.Set("timeFormat", "milliseconds")

	if !q.Since.IsZero() {
		v.Set("startDate", strconv.FormatInt(q.Since.UnixMilli(), 10))
	}
	if !q.Until.IsZero() {
		v.Set("endDate", strconv.FormatInt(q.Until.UnixMilli(), 10))
	}
	if q.MaxLevel > 0 {
		v.Set("maxLevel", strconv.Itoa(q.MaxLevel))
	}
	if q.TimeInterval != "" {
		v.Set("timeInterval", q.TimeInterval)
	}

	return v, nil
}

// MaxWindow is the largest span each endpoint family accepts. Exceeding it is a
// client error, so the paginator chunks against these rather than discovering
// them from a 400.
const (
	MaxTradesWindow    = 731 * 24 * time.Hour
	MaxOrderBookWindow = 18 * 30 * 24 * time.Hour
)

// get issues one request against refURL and returns the decoded page.
func get[T any](
	ctx context.Context, c *RestClient, refURL string, q Query,
) (Page[T], error) {
	values, err := q.values()
	if err != nil {
		return Page[T]{}, err
	}

	req, err := c.NewAuthenticatedRequest(ctx, "GET", refURL, values, nil)
	if err != nil {
		return Page[T]{}, err
	}

	return doPage[T](c, req)
}

// GetCursor continues a paginated result by re-issuing metadata.next verbatim.
func GetCursor[T any](ctx context.Context, c *RestClient, cursorURL string) (Page[T], error) {
	req, err := c.NewCursorRequest(ctx, cursorURL)
	if err != nil {
		return Page[T]{}, err
	}

	return doPage[T](c, req)
}

func doPage[T any](c *RestClient, req *http.Request) (Page[T], error) {
	var resp Response[T]
	if err := c.Do(req, &resp); err != nil {
		return Page[T]{}, err
	}

	return Page[T]{
		Data:          resp.Payload.Data,
		Next:          resp.Payload.Metadata.Next,
		ReturnedStart: resp.Payload.Metadata.ReturnedStartDate.Time,
		ReturnedEnd:   resp.Payload.Metadata.ReturnedEndDate.Time,
	}, nil
}

// instrumentPath builds "/markets/<class>/<endpoint>/<instrument>", escaping the
// instrument since some venues use characters that need it.
func instrumentPath(class AssetClass, endpoint, instrument string) string {
	return fmt.Sprintf("/markets/%s/%s/%s", class, endpoint, url.PathEscape(instrument))
}

// GetTrades fetches historical trades. The maximum window is 731 days.
func (c *RestClient) GetTrades(
	ctx context.Context, class AssetClass, q Query,
) (Page[Trade], error) {
	return get[Trade](ctx, c, instrumentPath(class, "trades", q.Instrument), q)
}

// GetOrderBookSnapshots fetches full order book states. The maximum window is
// 18 months.
func (c *RestClient) GetOrderBookSnapshots(
	ctx context.Context, class AssetClass, q Query,
) (Page[Book], error) {
	return get[Book](ctx, c, instrumentPath(class, "order-book-snapshots", q.Instrument), q)
}

// GetOrderBookEvents fetches incremental order book changes. Only changed levels
// appear, and a level with volume zero means it was removed.
func (c *RestClient) GetOrderBookEvents(
	ctx context.Context, class AssetClass, q Query,
) (Page[Book], error) {
	return get[Book](ctx, c, instrumentPath(class, "order-book-events", q.Instrument), q)
}

// GetOHLCV fetches candles. Set Query.TimeInterval to minutes, hours or days.
func (c *RestClient) GetOHLCV(
	ctx context.Context, class AssetClass, q Query,
) (Page[OHLCV], error) {
	return get[OHLCV](ctx, c, instrumentPath(class, "ohlcv", q.Instrument), q)
}

// GetTradesInformation reports the range each instrument has data for, which is
// how a backfill avoids requesting years that do not exist.
func (c *RestClient) GetTradesInformation(
	ctx context.Context, class AssetClass, q Query,
) (Page[InstrumentCoverage], error) {
	return get[InstrumentCoverage](ctx, c,
		fmt.Sprintf("/markets/%s/trades/information", class), q)
}

// GetExchangesReference reports contract metadata, including the contract size
// and position multiplier needed to interpret futures volumes.
func (c *RestClient) GetExchangesReference(
	ctx context.Context, class AssetClass, q Query,
) (Page[InstrumentReference], error) {
	return get[InstrumentReference](ctx, c,
		fmt.Sprintf("/markets/%s/exchanges/reference", class), q)
}

// NormalizeInstrument maps a bbgo symbol to the venue's instrument name.
//
// AmberData uses each venue's own naming, so a spot pair is lowercase with an
// underscore while a futures contract is upper case. Only the spot case needs
// transforming, and only when the caller passes a bbgo-style symbol.
func NormalizeInstrument(class AssetClass, symbol string) string {
	if class != AssetClassSpot {
		return strings.ToUpper(symbol)
	}
	if strings.Contains(symbol, "_") {
		return strings.ToLower(symbol)
	}
	return strings.ToLower(symbol)
}
