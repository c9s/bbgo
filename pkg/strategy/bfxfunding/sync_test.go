package bfxfunding

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
)

// mockPublicFundingTrades serves the public funding trades like Bitfinex does:
// ascending by time, the start parameter is inclusive and at most `limit` rows per page.
func mockPublicFundingTrades(t *testing.T, trades []FundingTrade, requests *int) *http.Client {
	transport := &httptesting.MockTransport{}
	transport.GET("/v2/trades/fUST/hist", func(req *http.Request) (*http.Response, error) {
		*requests++

		query := req.URL.Query()
		assert.Equal(t, "1", query.Get("sort"))

		start, err := strconv.ParseInt(query.Get("start"), 10, 64)
		require.NoError(t, err)

		limit, err := strconv.Atoi(query.Get("limit"))
		require.NoError(t, err)

		var rows [][]any
		for _, tr := range trades {
			if tr.Time.UnixMilli() < start {
				continue
			}

			if len(rows) == limit {
				break
			}

			rows = append(rows, []any{tr.ID, tr.Time.UnixMilli(), tr.Amount, tr.Rate, tr.Period})
		}

		if rows == nil {
			rows = [][]any{}
		}

		return httptesting.BuildResponseJson(http.StatusOK, rows), nil
	})

	return &http.Client{Transport: transport}
}

func TestTradeSyncer_Sync(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	// trades 3, 4, 5 share the same millisecond across the page boundary
	trades := []FundingTrade{
		{ID: 1, Time: t0, Amount: 100, Rate: 0.0001, Period: 2},
		{ID: 2, Time: t0.Add(time.Second), Amount: 100, Rate: 0.0001, Period: 2},
		{ID: 3, Time: t0.Add(2 * time.Second), Amount: 100, Rate: 0.0001, Period: 2},
		{ID: 4, Time: t0.Add(2 * time.Second), Amount: 100, Rate: 0.0001, Period: 2},
		{ID: 5, Time: t0.Add(2 * time.Second), Amount: 100, Rate: 0.0001, Period: 2},
		{ID: 6, Time: t0.Add(3 * time.Second), Amount: 100, Rate: 0.0002, Period: 30},
		{ID: 7, Time: t0.Add(4 * time.Second), Amount: 100, Rate: 0.0003, Period: 120},
	}

	requests := 0
	client := bfxapi.NewClient()
	client.HttpClient = mockPublicFundingTrades(t, trades, &requests)

	newSyncer := func() *TradeSyncer {
		syncer := NewTradeSyncer(client, "fUST", logrus.New())
		syncer.limiter = rate.NewLimiter(rate.Inf, 1)
		syncer.pageLimit = 3
		return syncer
	}

	collect := func(syncer *TradeSyncer, since time.Time) []FundingTrade {
		var got []FundingTrade
		n, err := syncer.Sync(context.Background(), since, func(page []FundingTrade) error {
			got = append(got, page...)
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, len(got), n)
		return got
	}

	ids := func(trades []FundingTrade) []int64 {
		var out []int64
		for _, tr := range trades {
			out = append(out, tr.ID)
		}

		slices.Sort(out)
		return out
	}

	t.Run("full sync without duplicates", func(t *testing.T) {
		got := collect(newSyncer(), t0)
		assert.Equal(t, []int64{1, 2, 3, 4, 5, 6, 7}, ids(got))
	})

	t.Run("since filters the older trades", func(t *testing.T) {
		got := collect(newSyncer(), t0.Add(2*time.Second))
		assert.Equal(t, []int64{3, 4, 5, 6, 7}, ids(got))
	})

	t.Run("resume from the cursor", func(t *testing.T) {
		syncer := newSyncer()
		syncer.SetCursor(t0.Add(2*time.Second), []int64{3, 4})
		got := collect(syncer, t0)
		assert.Equal(t, []int64{5, 6, 7}, ids(got))

		// nothing new on the next sync
		requests = 0
		got = collect(syncer, t0)
		assert.Empty(t, got)
		assert.Equal(t, 1, requests)
	})
}
