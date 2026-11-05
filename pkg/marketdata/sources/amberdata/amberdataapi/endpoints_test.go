package amberdataapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/testing/httptesting"
)

// The endpoints below were added after the first four. Each test pins the
// request path the slugs build and the fields its fixture — transcribed from the
// endpoint's own published response example — actually contains.

func TestGetFundingRates(t *testing.T) {
	client, captured := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport,
			"/markets/futures/funding-rates/BTCUSD_PERP", "futures_funding_rates.json")
	})

	resp, err := client.NewGetFundingRatesRequest(AssetClassFutures).
		Instrument("BTCUSD_PERP").Exchange("binance").Do(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "/markets/futures/funding-rates/BTCUSD_PERP", (*captured).URL.Path)

	page := pageOf(resp)
	require.Len(t, page.Data, 2)

	assert.Equal(t, "0.0001", page.Data[0].FundingRate.String())
	assert.True(t, page.Data[0].IsActualFundingRate,
		"a settled rate and a predicted one are not interchangeable")
	assert.False(t, page.Data[1].IsActualFundingRate)

	// Binance leaves these null.
	assert.False(t, page.Data[0].FundingInterval.Valid)
	assert.False(t, page.Data[0].NextFundingRate.Valid)
	assert.True(t, page.Data[0].NextFundingTime.IsZero())
}

func TestGetOpenInterest(t *testing.T) {
	client, captured := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport,
			"/markets/futures/open-interest/BTCUSD_PERP", "futures_open_interest.json")
	})

	resp, err := client.NewGetOpenInterestRequest(AssetClassFutures).
		Instrument("BTCUSD_PERP").Exchange("binance").Do(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "/markets/futures/open-interest/BTCUSD_PERP", (*captured).URL.Path)

	page := pageOf(resp)
	require.Len(t, page.Data, 1)

	assert.Equal(t, "16862174", page.Data[0].Value.String())
	assert.Equal(t, "PERPETUAL", page.Data[0].Type)
	assert.Contains(t, page.Next, "cursor=", "the cursor must be carried through verbatim")
}

func TestGetLiquidations(t *testing.T) {
	client, captured := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport,
			"/markets/futures/liquidations/BTCUSDT", "futures_liquidations.json")
	})

	resp, err := client.NewGetLiquidationsRequest(AssetClassFutures).
		Instrument("BTCUSDT").Exchange("binance").Do(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "/markets/futures/liquidations/BTCUSDT", (*captured).URL.Path)

	page := pageOf(resp)
	require.Len(t, page.Data, 1)

	liq := page.Data[0]
	assert.Equal(t, "70913.39", liq.Price.String())
	assert.Equal(t, "0.002", liq.Volume.String())

	// Side is the liquidating order's side; positionType is who was holding. A
	// short being closed out is bought back, so the two differ.
	assert.Equal(t, "BUY", liq.Side)
	assert.Equal(t, "SHORT", liq.PositionType)

	// unit matters: some venues report contracts rather than base asset.
	assert.Equal(t, "base_asset", liq.Unit)

	assert.Equal(t, "FILLED", liq.Status)
	assert.Equal(t, "IOC", liq.TimeInForce)
	assert.Equal(t, "LIMIT", liq.Type)
	assert.Empty(t, liq.OrderID, "binance leaves orderId null")
}

func TestGetLongShortRatio(t *testing.T) {
	client, captured := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport,
			"/markets/futures/long-short-ratio/BTCUSDT", "futures_long_short_ratio.json")
	})

	resp, err := client.NewGetLongShortRatioRequest(AssetClassFutures).
		Instrument("BTCUSDT").Exchange("binance").Do(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "/markets/futures/long-short-ratio/BTCUSDT", (*captured).URL.Path)

	page := pageOf(resp)
	require.Len(t, page.Data, 1)

	assert.Equal(t, "0.519", page.Data[0].LongAccount.String())
	assert.Equal(t, "0.481", page.Data[0].ShortAccount.String())
	assert.Equal(t, "1.079", page.Data[0].Ratio.String())
}

func TestGetTickers(t *testing.T) {
	client, captured := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport, "/markets/futures/tickers/BTCUSDT", "futures_tickers.json")
	})

	resp, err := client.NewGetTickersRequest(AssetClassFutures).
		Instrument("BTCUSDT").Exchange("binance").Do(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "/markets/futures/tickers/BTCUSDT", (*captured).URL.Path)

	page := pageOf(resp)
	require.Len(t, page.Data, 1)

	assert.Equal(t, "70836.3", page.Data[0].Bid.String())
	assert.Equal(t, "70836.4", page.Data[0].Ask.String())
	assert.Equal(t, "2.016", page.Data[0].BidVolume.String())
	assert.False(t, page.Data[0].Last.Valid, "binance leaves last null here")
}

// TestGetCoverage covers the information endpoints, which is how a backfill finds
// out what range an instrument actually has before requesting years of nothing.
// The feature is a slug, so one request type serves every family.
func TestGetCoverage(t *testing.T) {
	client, captured := newTestClient(t, func(transport *httptesting.MockTransport) {
		transport.GET("/markets/futures/trades/information",
			func(req *http.Request) (*http.Response, error) {
				return httptesting.BuildResponseString(http.StatusOK, `{
					"status":200,"title":"OK","description":"Successful request",
					"payload":{"metadata":{"next":null,"api-version":"2023-09-30"},
					"data":[{"exchange":"binance","instrument":"BTCUSDT",
					"startDate":1569888000000,"endDate":1717518000000}]}}`), nil
			})
	})

	resp, err := client.NewGetCoverageRequest(AssetClassFutures, FeatureTrades).
		Exchange("binance").Do(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "/markets/futures/trades/information", (*captured).URL.Path)

	page := pageOf(resp)
	require.Len(t, page.Data, 1)
	assert.Equal(t, "BTCUSDT", page.Data[0].Instrument)
	assert.Equal(t, int64(1569888000000), page.Data[0].StartDate.UnixMilli())
}

// TestGetCoverage_ExchangeIsOptional documents the difference from the data
// endpoints: omitting the exchange lists every exchange covered.
func TestGetCoverage_ExchangeIsOptional(t *testing.T) {
	client, captured := newTestClient(t, func(transport *httptesting.MockTransport) {
		transport.GET("/markets/futures/ohlcv/information",
			func(req *http.Request) (*http.Response, error) {
				return httptesting.BuildResponseString(http.StatusOK,
					`{"status":200,"payload":{"metadata":{},"data":[]}}`), nil
			})
	})

	_, err := client.NewGetCoverageRequest(AssetClassFutures, FeatureOHLCV).
		Do(context.Background())
	require.NoError(t, err)

	assert.Empty(t, (*captured).URL.Query().Get("exchange"))
}

// TestGetExchangesReference covers the endpoint that settles how to read a
// futures volume, via contractSize and underlyingToPositionMultiplier.
func TestGetExchangesReference(t *testing.T) {
	client, _ := newTestClient(t, func(transport *httptesting.MockTransport) {
		transport.GET("/markets/futures/exchanges/reference",
			func(req *http.Request) (*http.Response, error) {
				return httptesting.BuildResponseString(http.StatusOK, `{
					"status":200,"title":"OK","payload":{"metadata":{},
					"data":[{"exchange":"binance","instrument":"BTCUSDT",
					"baseSymbol":"BTC","quoteSymbol":"USDT",
					"contractPeriod":"perpetual","contractSettleType":"linear",
					"contractSize":1,"precisionPrice":0.1,"precisionVolume":0.001,
					"underlyingToPositionMultiplier":1}]}}`), nil
			})
	})

	resp, err := client.NewGetExchangesReferenceRequest(AssetClassFutures).
		Exchange("binance").Instrument("BTCUSDT").Do(context.Background())
	require.NoError(t, err)

	page := pageOf(resp)
	require.Len(t, page.Data, 1)

	ref := page.Data[0]
	assert.Equal(t, "perpetual", ref.ContractPeriod)
	assert.Equal(t, "linear", ref.ContractSettleType)
	assert.Equal(t, "1", ref.ContractSize.String())
	assert.Equal(t, "1", ref.UnderlyingToPositionMultiplier.String())
}

// TestSendRequest_PreservesAPIErrorClassification is the test that matters for
// the paginator: a generated Do must surface this package's APIError, not
// requestgen's own error type, or the window-shrinking logic cannot tell a
// too-large response from a forbidden one.
func TestSendRequest_PreservesAPIErrorClassification(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		shouldShrink bool
		forbidden    bool
	}{
		{
			name:         "400 over the size cap",
			status:       http.StatusBadRequest,
			body:         `{"status":400,"description":"payload size exceeds 10 MB"}`,
			shouldShrink: true,
		},
		{
			name:         "504 after the 28 second limit",
			status:       http.StatusGatewayTimeout,
			body:         `{}`,
			shouldShrink: true,
		},
		{
			name:      "403 from the gateway",
			status:    http.StatusForbidden,
			body:      `{"message":"Forbidden"}`,
			forbidden: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newTestClient(t, func(transport *httptesting.MockTransport) {
				transport.GET("/markets/futures/trades/BTCUSDT",
					func(req *http.Request) (*http.Response, error) {
						return httptesting.BuildResponseString(tt.status, tt.body), nil
					})
			})

			_, err := client.NewGetTradesRequest(AssetClassFutures).
				Instrument("BTCUSDT").Exchange("binance").Do(context.Background())
			require.Error(t, err)

			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr,
				"a generated Do must surface *APIError, not requestgen's error type")

			assert.Equal(t, tt.shouldShrink, apiErr.ShouldShrinkWindow())
			assert.Equal(t, tt.forbidden, apiErr.IsForbidden())
		})
	}
}
