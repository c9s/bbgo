package okex

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
)

// accountConfigJSON returns a minimal /api/v5/account/config payload.
func accountConfigJSON(acctLv int, autoLoan, enableSpotBorrow bool) *http.Response {
	return httptesting.BuildResponseString(http.StatusOK,
		`{"code":"0","data":[{"acctLv":`+itoa(acctLv)+
			`,"autoLoan":`+boolStr(autoLoan)+
			`,"enableSpotBorrow":`+boolStr(enableSpotBorrow)+`}]}`)
}

// accountBalanceJSON returns a minimal /api/v5/account/balance payload with the
// given adjusted equity, maintenance margin, and borrow notional (USD).
func accountBalanceJSON(adjEq, mmr, notionalUsdForBorrow, totalEq string) *http.Response {
	return httptesting.BuildResponseString(http.StatusOK,
		`{"code":"0","data":[{"totalEq":"`+totalEq+`","adjEq":"`+adjEq+
			`","mmr":"`+mmr+`","notionalUsdForBorrow":"`+notionalUsdForBorrow+`","details":[]}]}`)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

// Ticket 04: the margin level on a margin account reflects the real
// maintenance margin; a zero maintenance margin means "no debt yet" and is
// reported as the unlimited sentinel; non-margin accounts keep the
// total-equity / borrow-notional formula.
func TestExchange_QueryAccount_MarginLevel(t *testing.T) {
	t.Run("margin: level = adjusted equity / maintenance margin", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		// adjEq=50, mmr=10 -> margin level 5
		transport.GET("/api/v5/account/balance", func(*http.Request) (*http.Response, error) {
			return accountBalanceJSON("50", "10", "0", "100"), nil
		})
		transport.GET("/api/v5/account/config", func(*http.Request) (*http.Response, error) {
			return accountConfigJSON(3, true, false), nil
		})

		account, err := ex.QueryAccount(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, fixedpoint.MustNewFromString("5"), account.MarginLevel)
		// cross mode: borrowing is governed by auto loan, not spot borrow
		assert.True(t, *account.BorrowEnabled)
	})

	t.Run("margin: zero maintenance margin -> unlimited sentinel", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		// mmr=0 means no cross debt yet -> level must be the max sentinel, never 0
		transport.GET("/api/v5/account/balance", func(*http.Request) (*http.Response, error) {
			return accountBalanceJSON("100", "0", "0", "100"), nil
		})
		transport.GET("/api/v5/account/config", func(*http.Request) (*http.Response, error) {
			return accountConfigJSON(3, true, false), nil
		})

		account, err := ex.QueryAccount(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, defaultMaxMarginLevel, account.MarginLevel)
	})

	t.Run("spot with borrow: level = total equity / borrow notional (unchanged)", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		// IsMargin false, but spot borrow enabled

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		// totalEq=200, notionalUsdForBorrow=40 -> margin level 5 (spot formula)
		transport.GET("/api/v5/account/balance", func(*http.Request) (*http.Response, error) {
			return accountBalanceJSON("50", "0", "40", "200"), nil
		})
		transport.GET("/api/v5/account/config", func(*http.Request) (*http.Response, error) {
			return accountConfigJSON(1, false, true), nil
		})

		account, err := ex.QueryAccount(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, fixedpoint.MustNewFromString("5"), account.MarginLevel)
		// spot mode: borrowing is governed by enableSpotBorrow
		assert.True(t, *account.BorrowEnabled)
	})
}

// Ticket 04 (guard): CheckMarginAccount fails fast with a descriptive error
// when the account is below level 3 or auto loan is off; it is a no-op when
// margin is not enabled and succeeds for a properly configured account.
func TestExchange_CheckMarginAccount(t *testing.T) {
	t.Run("margin off: no-op, no config query", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		// IsMargin false

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport
		// no handlers registered: any config query would fail the test

		assert.NoError(t, ex.CheckMarginAccount(context.Background()))
	})

	t.Run("level 3 + auto loan: succeeds", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport
		transport.GET("/api/v5/account/config", func(*http.Request) (*http.Response, error) {
			return accountConfigJSON(3, true, false), nil
		})

		assert.NoError(t, ex.CheckMarginAccount(context.Background()))
	})

	t.Run("level below 3: names the account-level problem", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport
		transport.GET("/api/v5/account/config", func(*http.Request) (*http.Response, error) {
			return accountConfigJSON(1, true, false), nil
		})

		err := ex.CheckMarginAccount(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "acctLv")
		assert.Contains(t, err.Error(), "got acctLv=1")
	})

	t.Run("auto loan off: names the auto-loan problem", func(t *testing.T) {
		ex := New("key", "secret", "passphrase")
		ex.MarginSettings.IsMargin = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport
		transport.GET("/api/v5/account/config", func(*http.Request) (*http.Response, error) {
			return accountConfigJSON(3, false, false), nil
		})

		err := ex.CheckMarginAccount(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "auto loan")
	})
}
