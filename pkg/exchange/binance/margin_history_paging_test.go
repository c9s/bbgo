package binance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
)

func fakeInterestPages(sizes ...int) (func(ctx context.Context, page int) ([]binanceapi.MarginInterest, error), *[]int) {
	var calls []int
	return func(_ context.Context, page int) ([]binanceapi.MarginInterest, error) {
		calls = append(calls, page)
		if page > len(sizes) {
			return nil, nil
		}
		return make([]binanceapi.MarginInterest, sizes[page-1]), nil
	}, &calls
}

func TestQueryInterestPages(t *testing.T) {
	ctx := context.Background()

	t.Run("pages until a short page", func(t *testing.T) {
		fetch, calls := fakeInterestPages(100, 100, 50)
		rows, err := queryInterestPages(ctx, fetch)
		assert.NoError(t, err)
		assert.Len(t, rows, 250)
		assert.Equal(t, []int{1, 2, 3}, *calls)
	})

	t.Run("empty first page", func(t *testing.T) {
		fetch, calls := fakeInterestPages(0)
		rows, err := queryInterestPages(ctx, fetch)
		assert.NoError(t, err)
		assert.Empty(t, rows)
		assert.Equal(t, []int{1}, *calls)
	})

	t.Run("exactly one full page", func(t *testing.T) {
		fetch, calls := fakeInterestPages(100)
		rows, err := queryInterestPages(ctx, fetch)
		assert.NoError(t, err)
		assert.Len(t, rows, 100)
		assert.Equal(t, []int{1, 2}, *calls)
	})

	t.Run("page cap returns no rows", func(t *testing.T) {
		sizes := make([]int, interestHistoryMaxPages+5)
		for i := range sizes {
			sizes[i] = interestHistoryPageSize
		}
		fetch, calls := fakeInterestPages(sizes...)
		rows, err := queryInterestPages(ctx, fetch)
		assert.ErrorIs(t, err, errInterestPageCap)
		assert.Nil(t, rows)
		assert.Len(t, *calls, interestHistoryMaxPages)
	})

	t.Run("error", func(t *testing.T) {
		_, err := queryInterestPages(ctx, func(context.Context, int) ([]binanceapi.MarginInterest, error) {
			return nil, errors.New("boom")
		})
		assert.Error(t, err)
	})
}

// fakeInterestWindow serves rowsPerHour rows for every hour of [start, end]
func fakeInterestWindow(start time.Time, rowsPerHour int) (func(context.Context, *time.Time, int) ([]binanceapi.MarginInterest, error), *[]time.Time) {
	var ends []time.Time
	return func(_ context.Context, end *time.Time, page int) ([]binanceapi.MarginInterest, error) {
		if page == 1 {
			ends = append(ends, *end)
		}
		total := int(end.Sub(start).Minutes()) * rowsPerHour / 60
		from := (page - 1) * interestHistoryPageSize
		n := min(max(total-from, 0), interestHistoryPageSize)
		return make([]binanceapi.MarginInterest, n), nil
	}, &ends
}

func TestQueryInterestWindow(t *testing.T) {
	ctx := context.Background()
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)

	t.Run("a window that fits is fetched once", func(t *testing.T) {
		fetch, ends := fakeInterestWindow(start, 1)
		rows, err := queryInterestWindow(ctx, &start, &end, fetch)
		assert.NoError(t, err)
		assert.Len(t, rows, 720)
		assert.Equal(t, []time.Time{end}, *ends)
	})

	t.Run("a window over the cap is halved until it fits", func(t *testing.T) {
		// 30 rows per hour: 21600 rows in 30 days, 10800 in 15, 5400 in 7.5
		fetch, ends := fakeInterestWindow(start, 30)
		rows, err := queryInterestWindow(ctx, &start, &end, fetch)
		assert.NoError(t, err)
		assert.Len(t, rows, 5400)
		assert.Equal(t, []time.Time{end, start.Add(15 * 24 * time.Hour), start.Add(180 * time.Hour)}, *ends)
	})

	t.Run("a window that is still over the cap at an hour is a permanent error", func(t *testing.T) {
		fetch, _ := fakeInterestWindow(start, 3*interestHistoryMaxPages*interestHistoryPageSize)
		_, err := queryInterestWindow(ctx, &start, &end, fetch)
		assert.ErrorIs(t, err, errInterestPageCap)

		// the batch query must not retry the same window
		var permanent *backoff.PermanentError
		assert.ErrorAs(t, err, &permanent)
	})
}
