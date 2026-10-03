package okex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
)

type repayBody struct {
	Ccy        string `json:"ccy"`
	Side       string `json:"side"`
	Am         string `json:"amt"`
	DebtCcy    string `json:"debtCcy"`
	RepayCcy   string `json:"repayCcy"`
}

func readRepayBody(t *testing.T, req *http.Request) repayBody {
	t.Helper()
	body, err := io.ReadAll(req.Body)
	assert.NoError(t, err)
	var b repayBody
	assert.NoError(t, json.Unmarshal(body, &b))
	return b
}

// Ticket 05: repayment routes per account mode. A margin account settles a
// non-quote debt via one-click-repay (debtCcy + repayCcy=USDT); a USDT debt is
// skipped with no call; a spot account uses the existing spot borrow/repay
// endpoint unchanged.
func TestExchange_RepayMarginAsset_Routing(t *testing.T) {
	amount := fixedpoint.MustNewFromString("0.5")

	t.Run("margin: non-quote debt -> one-click-repay with USDT", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		oneClickCalled := false
		transport.POST("/api/v5/trade/one-click-repay", func(req *http.Request) (*http.Response, error) {
			oneClickCalled = true
			b := readRepayBody(t, req)
			assert.Equal(t, "BTC", b.DebtCcy)
			assert.Equal(t, "USDT", b.RepayCcy)
			// no amount parameter: one-click-repay repays in full
			return httptesting.BuildResponseString(http.StatusOK,
				`{"code":"0","data":[{"debtCcy":"BTC","repayCcy":"USDT","repayAmt":"0.5"}]}`), nil
		})

		// the spot endpoint must NOT be touched in margin mode
		transport.POST("/api/v5/account/spot-manual-borrow-repay", func(req *http.Request) (*http.Response, error) {
			t.Fatal("spot borrow/repay endpoint must not be used in margin mode")
			return nil, nil
		})

		err := ex.RepayMarginAsset(context.Background(), "BTC", amount)
		assert.NoError(t, err)
		assert.True(t, oneClickCalled)
	})

	t.Run("margin: USDT debt -> skip, no repayment call", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport
		// no handlers registered: any repayment call would fail the test

		err := ex.RepayMarginAsset(context.Background(), "USDT", amount)
		assert.NoError(t, err, "a USDT debt is a documented skip, not an error")
	})

	t.Run("spot: existing spot borrow/repay endpoint, unchanged", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		// IsMargin false (spot)

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		spotCalled := false
		transport.POST("/api/v5/account/spot-manual-borrow-repay", func(req *http.Request) (*http.Response, error) {
			spotCalled = true
			b := readRepayBody(t, req)
			assert.Equal(t, "BTC", b.Ccy)
			assert.Equal(t, "repay", b.Side)
			assert.Equal(t, "0.5", b.Am)
			return httptesting.BuildResponseString(http.StatusOK,
				`{"code":"0","data":[{"ccy":"BTC","side":"repay","amt":"0.5"}]}`), nil
		})

		err := ex.RepayMarginAsset(context.Background(), "BTC", amount)
		assert.NoError(t, err)
		assert.True(t, spotCalled)
	})

	t.Run("margin: one-click-repay failure propagates", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		transport.POST("/api/v5/trade/one-click-repay", func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusOK,
				`{"code":"50011","msg":"invalid ccy","data":[]}`), nil
		})

		err := ex.RepayMarginAsset(context.Background(), "BTC", amount)
		assert.Error(t, err)
	})
}
