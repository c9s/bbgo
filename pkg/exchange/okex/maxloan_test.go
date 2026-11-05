package okex

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
)

// Ticket 01: the max-loan query must use the instrument id for margin
// accounts and the currency for spot accounts. The regression being guarded
// is 50014 "instrument ID cannot be empty", which kept the hedge box closed
// because a margin account rejects the ccy-only query.
func TestExchange_QueryMarginAssetMaxBorrowable(t *testing.T) {
	t.Run("margin account sends instId, not ccy", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		transport.GET("/api/v5/account/max-loan", func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Equal(t, []string{"BTC-USDT"}, query["instId"])
			// ccy must not be sent for a margin account
			assert.NotContains(t, query, "ccy")
			assert.Equal(t, []string{"cross"}, query["mgnMode"])

			return httptesting.BuildResponseString(http.StatusOK,
				`{"code":"0","data":[{"instId":"BTC-USDT","mgnMode":"cross","maxLoan":"1.5","ccy":"BTC"}]}`), nil
		})

		maxLoan, err := ex.QueryMarginAssetMaxBorrowable(context.Background(), "BTC")
		assert.NoError(t, err)
		assert.Equal(t, fixedpoint.MustNewFromString("1.5"), maxLoan)
	})

	t.Run("spot account sends ccy, not instId", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		// IsMargin defaults to false (spot)

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		transport.GET("/api/v5/account/max-loan", func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Equal(t, []string{"BTC"}, query["ccy"])
			// instId must not be sent for a spot account
			assert.NotContains(t, query, "instId")

			return httptesting.BuildResponseString(http.StatusOK,
				`{"code":"0","data":[{"maxLoan":"2.0","ccy":"BTC"}]}`), nil
		})

		maxLoan, err := ex.QueryMarginAssetMaxBorrowable(context.Background(), "BTC")
		assert.NoError(t, err)
		assert.Equal(t, fixedpoint.MustNewFromString("2"), maxLoan)
	})

	t.Run("empty response yields zero loan, no error", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		transport.GET("/api/v5/account/max-loan", func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusOK, `{"code":"0","data":[]}`), nil
		})

		maxLoan, err := ex.QueryMarginAssetMaxBorrowable(context.Background(), "BTC")
		assert.NoError(t, err)
		assert.True(t, maxLoan.IsZero())
	})
}
