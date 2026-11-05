package amberdataapi

import (
	"context"
	"fmt"

	"strings"
	"time"
)

// AssetClass selects the /futures or /spot family of endpoints. Both expose the
// same features under a different prefix, so it is a slug on every request type
// rather than a duplicated set of them.
type AssetClass string

const (
	AssetClassFutures AssetClass = "futures"
	AssetClassSpot    AssetClass = "spot"
)

// TimeFormatMilliseconds is the only time format this client sends.
//
// The server's default is `hr`, which emits "2024-06-04 16:23:12 414" — a
// space-separated millisecond field that is neither RFC3339 nor a number. It is
// set on every request builder by its constructor and is not exposed as an
// option, because making it configurable would only create a way to get it
// wrong.
const TimeFormatMilliseconds = "milliseconds"

// Feature names for GetCoverageRequest.
const (
	FeatureTrades             = "trades"
	FeatureOrderBookSnapshots = "order-book-snapshots"
	FeatureOHLCV              = "ohlcv"
	FeatureFundingRates       = "funding-rates"
	FeatureOpenInterest       = "open-interest"
	FeatureLiquidations       = "liquidations"
	FeatureLongShortRatio     = "long-short-ratio"
	FeatureTickers            = "tickers"
)

// Maximum windows each family accepts. The paginator chunks against these rather
// than discovering them from a 400.
//
// Note a contradiction in the published specifications: the trades page warns
// that the maximum range is 731 days, while its own startDate parameter
// description says the default window is one hour with a maximum of one hour.
// The former is treated as the real limit.
const (
	MaxTradesWindow    = 731 * 24 * time.Hour
	MaxOrderBookWindow = 18 * 30 * 24 * time.Hour
)

// Page is one page of results plus the cursor that continues it.
type Page[T any] struct {
	Data []T

	// Next is metadata.next verbatim: a complete absolute URL carrying an opaque
	// compressed cursor. It must be re-issued as-is; it cannot be reconstructed.
	// Empty when the result set is exhausted.
	Next string

	// ReturnedStart and ReturnedEnd are the range the server actually served,
	// which can be narrower than the one requested.
	ReturnedStart time.Time
	ReturnedEnd   time.Time
}

// pageOf converts a response envelope into a Page.
func pageOf[T any](resp *Response[T]) Page[T] {
	if resp == nil {
		return Page[T]{}
	}

	return Page[T]{
		Data:          resp.Payload.Data,
		Next:          resp.Payload.Metadata.Next,
		ReturnedStart: resp.Payload.Metadata.ReturnedStartDate.Time,
		ReturnedEnd:   resp.Payload.Metadata.ReturnedEndDate.Time,
	}
}

// Request constructors. Each sets the asset class and the time format, so a
// caller cannot forget either.

func (c *RestClient) NewGetTradesRequest(class AssetClass) *GetTradesRequest {
	return &GetTradesRequest{client: c, assetClass: class, timeFormat: TimeFormatMilliseconds}
}

func (c *RestClient) NewGetOrderBookSnapshotsRequest(class AssetClass) *GetOrderBookSnapshotsRequest {
	return &GetOrderBookSnapshotsRequest{client: c, assetClass: class, timeFormat: TimeFormatMilliseconds}
}

func (c *RestClient) NewGetOrderBookEventsRequest(class AssetClass) *GetOrderBookEventsRequest {
	return &GetOrderBookEventsRequest{client: c, assetClass: class, timeFormat: TimeFormatMilliseconds}
}

func (c *RestClient) NewGetOHLCVRequest(class AssetClass) *GetOHLCVRequest {
	return &GetOHLCVRequest{client: c, assetClass: class, timeFormat: TimeFormatMilliseconds}
}

func (c *RestClient) NewGetFundingRatesRequest(class AssetClass) *GetFundingRatesRequest {
	return &GetFundingRatesRequest{client: c, assetClass: class, timeFormat: TimeFormatMilliseconds}
}

func (c *RestClient) NewGetOpenInterestRequest(class AssetClass) *GetOpenInterestRequest {
	return &GetOpenInterestRequest{client: c, assetClass: class, timeFormat: TimeFormatMilliseconds}
}

func (c *RestClient) NewGetLiquidationsRequest(class AssetClass) *GetLiquidationsRequest {
	return &GetLiquidationsRequest{client: c, assetClass: class, timeFormat: TimeFormatMilliseconds}
}

func (c *RestClient) NewGetLongShortRatioRequest(class AssetClass) *GetLongShortRatioRequest {
	return &GetLongShortRatioRequest{client: c, assetClass: class, timeFormat: TimeFormatMilliseconds}
}

func (c *RestClient) NewGetTickersRequest(class AssetClass) *GetTickersRequest {
	return &GetTickersRequest{client: c, assetClass: class, timeFormat: TimeFormatMilliseconds}
}

func (c *RestClient) NewGetCoverageRequest(class AssetClass, feature string) *GetCoverageRequest {
	return &GetCoverageRequest{
		client: c, assetClass: class, feature: feature, timeFormat: TimeFormatMilliseconds,
	}
}

func (c *RestClient) NewGetExchangesReferenceRequest(class AssetClass) *GetExchangesReferenceRequest {
	return &GetExchangesReferenceRequest{client: c, assetClass: class, timeFormat: TimeFormatMilliseconds}
}

// GetCursor continues a paginated result by re-issuing metadata.next verbatim.
//
// It is a free function rather than a request builder because the cursor is an
// opaque URL: there are no parameters to set, and appending any would risk
// invalidating it.
func GetCursor[T any](ctx context.Context, c *RestClient, cursorURL string) (Page[T], error) {
	if cursorURL == "" {
		return Page[T]{}, fmt.Errorf("amberdata: empty cursor url")
	}
	if !strings.HasPrefix(cursorURL, "http://") && !strings.HasPrefix(cursorURL, "https://") {
		// A relative cursor would silently resolve against the wrong host.
		return Page[T]{}, fmt.Errorf("amberdata: cursor url %q is not absolute", cursorURL)
	}

	req, err := c.NewCursorRequest(ctx, cursorURL)
	if err != nil {
		return Page[T]{}, err
	}

	resp, err := c.SendRequest(req)
	if err != nil {
		return Page[T]{}, err
	}

	var envelope Response[T]
	if err := resp.DecodeJSON(&envelope); err != nil {
		return Page[T]{}, fmt.Errorf("amberdata: decoding cursor response: %w", err)
	}

	return pageOf(&envelope), nil
}

// NormalizeInstrument maps a bbgo symbol to the venue's instrument name.
//
// AmberData uses each venue's own naming: a futures contract is upper case
// (BTCUSDT, BTCUSD_PERP) while a spot pair is lower case with an underscore
// (btc_usd).
func NormalizeInstrument(class AssetClass, symbol string) string {
	if class == AssetClassSpot {
		return strings.ToLower(symbol)
	}
	return strings.ToUpper(symbol)
}

// optionalTime is a helper for callers holding a possibly-zero time.
func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// DoPage runs a generated request builder's Do and converts its envelope into a
// Page.
//
// It takes the method value rather than the request, because each builder is a
// distinct generated type with its own Do signature; this is the one place that
// has to know the envelope is wrapped.
func DoPage[T any](ctx context.Context, do func(context.Context) (*Response[T], error)) (Page[T], error) {
	resp, err := do(ctx)
	if err != nil {
		return Page[T]{}, err
	}
	return pageOf(resp), nil
}
