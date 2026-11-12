package batch

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/types"
)

type fakeInterestHistory struct {
	types.MarginHistoryService
	rows []types.MarginInterest
}

func (f *fakeInterestHistory) QueryInterestHistory(_ context.Context, _ string, start, end *time.Time) ([]types.MarginInterest, error) {
	var out []types.MarginInterest
	for _, r := range f.rows {
		if t := r.Time.Time(); !t.Before(*start) && !t.After(*end) {
			out = append(out, r)
		}
	}
	return out, nil
}

func TestMarginInterestBatchQuery_KeepsRowsOfTheSameHour(t *testing.T) {
	h := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	// one BNB-paid hour has a row per borrowed asset, all at the same time
	q := &MarginInterestBatchQuery{MarginHistoryService: &fakeInterestHistory{rows: []types.MarginInterest{
		{Asset: "BNB", RawAsset: "BTC", Time: types.Time(h)},
		{Asset: "BNB", RawAsset: "ETH", Time: types.Time(h)},
		{Asset: "BNB", RawAsset: "BTC", Time: types.Time(h.Add(time.Hour))},
	}}}

	c, errC := q.Query(context.Background(), "BNB", h.Add(-time.Hour), h.Add(2*time.Hour))

	var got []string
	for m := range c {
		got = append(got, m.Key())
	}
	require.NoError(t, <-errC)
	assert.ElementsMatch(t, []string{
		"BNB/BTC//" + "1794736800000",
		"BNB/ETH//" + "1794736800000",
		"BNB/BTC//" + "1794740400000",
	}, got)
}
