package xmaker

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/okex"
	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

// okexHedgeSession builds a real *okex.Exchange (so the *okex.Exchange type
// assertion in the guard matches) pointed at a mock transport, wired into an
// okex session with margin enabled. The account-config response is what the
// guard (okex.EnsureAutoLoanEnabled) will observe.
func okexHedgeSession(t *testing.T, acctLv int, autoLoan bool) (*bbgo.ExchangeSession, *httptesting.MockTransport) {
	t.Helper()
	transport := &httptesting.MockTransport{}
	client := okexapi.NewClient()
	// dummy credentials so the signed account-config request passes the
	// client's auth check and reaches the mock transport
	client.Auth("key", "secret", "passphrase")
	client.HttpClient.Transport = transport

	ex := okex.New("key", "secret", "passphrase", okex.WithClient(client))
	ex.MarginSettings.IsMargin = true

	transport.GET("/api/v5/account/config", func(*http.Request) (*http.Response, error) {
		return httptesting.BuildResponseString(http.StatusOK,
			`{"code":"0","data":[{"acctLv":`+strconv.Itoa(acctLv)+
				`,"autoLoan":`+strconv.FormatBool(autoLoan)+`,"enableSpotBorrow":false}]}`), nil
	})

	hedge := bbgo.NewExchangeSession("okex", ex)
	hedge.Margin = true
	return hedge, transport
}

// mockHedgeSession builds a hedge session whose exchange is NOT *okex.Exchange
// (e.g. binance / max), so the guard's type assertion misses and the probe is
// a no-op.
func mockHedgeSession(t *testing.T) *bbgo.ExchangeSession {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	ex := mocks.NewMockExchange(ctrl)
	ex.EXPECT().NewStream().Return(&types.StandardStream{}).AnyTimes()
	ex.EXPECT().Name().Return(types.ExchangeName("okex")).AnyTimes()

	hedge := bbgo.NewExchangeSession("okex", ex)
	hedge.Margin = true
	return hedge
}

func crossRunSessions(hedge *bbgo.ExchangeSession) map[string]*bbgo.ExchangeSession {
	return map[string]*bbgo.ExchangeSession{
		"okex": hedge,
		"max":  &bbgo.ExchangeSession{},
	}
}

func crossRunStrategy(hedge *bbgo.ExchangeSession) *Strategy {
	return &Strategy{
		StrategyConfig: StrategyConfig{
			SourceSymbol:   "BTCUSDT",
			SourceExchange: "okex",
			MakerExchange:  "max",
		},
		Symbol: "BTCUSDT",
		logger: logrus.New(),
	}
}

// marker that CrossRun passed the account guard and continued to the (missing)
// hedge market lookup.
const marketNotDefined = "source session market BTCUSDT is not defined"

// Ticket 06: a misconfigured OKX hedge account disables only this strategy
// instance (no error propagates, so bbgo keeps running), while a good account,
// an exchange without the guard, and a margin-disabled session all proceed
// past the guard.
func TestStrategy_CrossRun_MarginAccountGuard(t *testing.T) {
	t.Run("guard fails: this instance does not start, no error propagates", func(t *testing.T) {
		// acctLv=1 -> EnsureAutoLoanEnabled errors -> the guard logs and
		// returns nil (instance blocked) instead of an error that would take
		// down the whole bbgo process.
		hedge, _ := okexHedgeSession(t, 1, true)

		s := crossRunStrategy(hedge)
		err := s.CrossRun(context.Background(), nil, crossRunSessions(hedge))

		assert.NoError(t, err, "a misconfigured okex account must not take down bbgo")
	})

	t.Run("guard succeeds: proceeds past the guard to the market lookup", func(t *testing.T) {
		// acctLv=3 + autoLoan -> guard passes; no market configured on the
		// hedge session, so CrossRun advances to the first market lookup.
		hedge, _ := okexHedgeSession(t, 3, true)

		s := crossRunStrategy(hedge)
		err := s.CrossRun(context.Background(), nil, crossRunSessions(hedge))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), marketNotDefined, "a good account must proceed past the guard")
		assert.NotContains(t, err.Error(), "acctLv")
	})

	t.Run("guard query fails: this instance does not start, no error propagates", func(t *testing.T) {
		// A transient API failure while querying the account config must
		// behave like any other guard failure: this instance is disabled
		// (CrossRun returns nil) instead of aborting Trader.Run and stopping
		// the whole bbgo process.
		hedge, transport := okexHedgeSession(t, 3, true)
		configQueries := 0
		transport.GET("/api/v5/account/config", func(*http.Request) (*http.Response, error) {
			configQueries++
			return nil, errors.New("connection refused")
		})

		s := crossRunStrategy(hedge)
		err := s.CrossRun(context.Background(), nil, crossRunSessions(hedge))

		assert.Equal(t, 1, configQueries, "the guard must attempt the config query")
		assert.NoError(t, err, "a config query failure must not take down bbgo")
	})

	t.Run("exchange without the guard: probe is a no-op and proceeds", func(t *testing.T) {
		hedge := mockHedgeSession(t)

		s := crossRunStrategy(hedge)
		err := s.CrossRun(context.Background(), nil, crossRunSessions(hedge))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), marketNotDefined, "a non-okex exchange must proceed unchanged")
	})

	t.Run("margin disabled: guard is never invoked", func(t *testing.T) {
		hedge, transport := okexHedgeSession(t, 1, true)
		hedge.Margin = false

		configQueries := 0
		transport.GET("/api/v5/account/config", func(*http.Request) (*http.Response, error) {
			configQueries++
			return httptesting.BuildResponseString(http.StatusOK,
				`{"code":"0","data":[{"acctLv":"1","autoLoan":true,"enableSpotBorrow":false}]}`), nil
		})

		s := crossRunStrategy(hedge)
		err := s.CrossRun(context.Background(), nil, crossRunSessions(hedge))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), marketNotDefined)
		assert.Equal(t, 0, configQueries, "the guard must not run when margin is disabled")
	})
}
