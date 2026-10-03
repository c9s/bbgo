package okex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/types"
)

// orderBodyParams is the subset of the /trade/order request body the sizing
// decision (ADR-0001) cares about. tgtCcy is absent (not nil) for margin
// market orders.
type orderBodyParams struct {
	TdMode string `json:"tdMode"`
	Side   string `json:"side"`
	OrdTyp string `json:"ordType"`
	Sz     string `json:"sz"`
	TgtCcy string `json:"tgtCcy"`
}

func testSizingMarket() types.Market {
	return types.Market{
		Symbol:          "BTCUSDT",
		BaseCurrency:    "BTC",
		QuoteCurrency:   "USDT",
		StepSize:        fixedpoint.MustNewFromString("0.0001"),
		TickSize:        fixedpoint.MustNewFromString("0.01"),
		PricePrecision:  2,
		VolumePrecision: 8,
		QuotePrecision:  2,
	}
}

// readOrderBody decodes the /trade/order request body into orderBodyParams.
func readOrderBody(t *testing.T, req *http.Request) orderBodyParams {
	t.Helper()
	body, err := io.ReadAll(req.Body)
	assert.NoError(t, err)
	var params orderBodyParams
	assert.NoError(t, json.Unmarshal(body, &params))
	return params
}

// respondOrder is a minimal valid /trade/order response.
func respondOrder() *http.Response {
	return httptesting.BuildResponseString(http.StatusOK,
		`{"code":"0","data":[{"ordId":"123456789","sCode":"0","sMsg":""}]}`)
}

// respondTicker returns a market ticker with the given best ask price.
func respondTicker(ask string) *http.Response {
	return httptesting.BuildResponseString(http.StatusOK,
		`{"code":"0","data":[{"instId":"BTC-USDT","last":"99999","askPx":"`+ask+`","bidPx":"99998"}]}`)
}

// Ticket 02 (ADR-0001): margin market orders must send no tgtCcy, a margin
// sell passes the base quantity through, and a margin buy is converted to the
// quote notional at the best ask.
func TestExchange_SubmitOrder_MarginMarketSizing(t *testing.T) {
	market := testSizingMarket()
	qty := fixedpoint.MustNewFromString("0.01")

	t.Run("margin sell: base size, no tgtCcy", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		var got orderBodyParams
		transport.POST("/api/v5/trade/order", func(req *http.Request) (*http.Response, error) {
			got = readOrderBody(t, req)
			return respondOrder(), nil
		})

		_, err := ex.SubmitOrder(context.Background(), types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeMarket,
			Quantity: qty,
			Market:   market,
		})
		assert.NoError(t, err)

		assert.Equal(t, "cross", got.TdMode)
		assert.Equal(t, "sell", got.Side)
		assert.Equal(t, "market", got.OrdTyp)
		// base-denominated, formatted at the step size (0.0001 -> 4dp)
		assert.Equal(t, "0.0100", got.Sz)
		assert.Empty(t, got.TgtCcy, "margin market sell must not send tgtCcy")
	})

	t.Run("margin buy: quote notional at best ask, no tgtCcy", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		transport.GET("/api/v5/market/ticker", func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, []string{"BTC-USDT"}, req.URL.Query()["instId"])
			return respondTicker("100000"), nil
		})

		var got orderBodyParams
		transport.POST("/api/v5/trade/order", func(req *http.Request) (*http.Response, error) {
			got = readOrderBody(t, req)
			return respondOrder(), nil
		})

		_, err := ex.SubmitOrder(context.Background(), types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: qty,
			Market:   market,
		})
		assert.NoError(t, err)

		assert.Equal(t, "cross", got.TdMode)
		assert.Equal(t, "buy", got.Side)
		// 0.01 BTC * 100000 USDT = 1000.00 USDT, at quote precision 2
		assert.Equal(t, "1000.00", got.Sz)
		assert.Empty(t, got.TgtCcy, "margin market buy must not send tgtCcy")
	})

	t.Run("margin buy with zero ask: explicit error, no order sent", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		transport.GET("/api/v5/market/ticker", func(req *http.Request) (*http.Response, error) {
			return respondTicker("0"), nil
		})

		orderSent := false
		transport.POST("/api/v5/trade/order", func(req *http.Request) (*http.Response, error) {
			orderSent = true
			return respondOrder(), nil
		})

		_, err := ex.SubmitOrder(context.Background(), types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: qty,
			Market:   market,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ask price is zero")
		assert.False(t, orderSent, "no order may be sent when the ask is zero")
	})
}

// Non-margin modes must be untouched by the sizing change.
func TestExchange_SubmitOrder_NonMarginMarketSizing(t *testing.T) {
	market := testSizingMarket()
	qty := fixedpoint.MustNewFromString("0.01")

	t.Run("spot market: base size, tgtCcy base", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		// IsMargin false, IsFutures false (spot)

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		var got orderBodyParams
		transport.POST("/api/v5/trade/order", func(req *http.Request) (*http.Response, error) {
			got = readOrderBody(t, req)
			return respondOrder(), nil
		})

		_, err := ex.SubmitOrder(context.Background(), types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeMarket,
			Quantity: qty,
			Market:   market,
		})
		assert.NoError(t, err)

		assert.Equal(t, "cash", got.TdMode)
		assert.Equal(t, "0.0100", got.Sz)
		assert.Equal(t, string(okexapi.TargetCurrencyBase), got.TgtCcy)
	})

	t.Run("futures market: base size, no tgtCcy", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.IsFutures = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		var got orderBodyParams
		transport.POST("/api/v5/trade/order", func(req *http.Request) (*http.Response, error) {
			got = readOrderBody(t, req)
			return respondOrder(), nil
		})

		_, err := ex.SubmitOrder(context.Background(), types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeMarket,
			Quantity: qty,
			Market:   market,
		})
		assert.NoError(t, err)

		// futures sz is contract-count semantics; the sizing branch must be
		// skipped (no quote conversion, no tgtCcy)
		assert.Equal(t, "0.0100", got.Sz)
		assert.Empty(t, got.TgtCcy)
	})
}
