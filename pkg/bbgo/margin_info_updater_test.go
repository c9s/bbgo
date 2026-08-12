package bbgo

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// mockMarginBorrowRepayService is a stub types.MarginBorrowRepayService whose
// QueryMarginAssetMaxBorrowable returns a preconfigured amount per asset (and an
// optional error), while recording how many times each asset was queried.
type mockMarginBorrowRepayService struct {
	mu sync.Mutex

	maxBorrowable map[string]fixedpoint.Value
	errs          map[string]error
	queryCount    map[string]int
}

func newMockMarginBorrowRepayService(maxBorrowable map[string]fixedpoint.Value) *mockMarginBorrowRepayService {
	return &mockMarginBorrowRepayService{
		maxBorrowable: maxBorrowable,
		errs:          make(map[string]error),
		queryCount:    make(map[string]int),
	}
}

func (m *mockMarginBorrowRepayService) RepayMarginAsset(
	_ context.Context, _ string, _ fixedpoint.Value,
) error {
	return nil
}

func (m *mockMarginBorrowRepayService) BorrowMarginAsset(
	_ context.Context, _ string, _ fixedpoint.Value,
) error {
	return nil
}

func (m *mockMarginBorrowRepayService) QueryMarginAssetMaxBorrowable(
	_ context.Context, asset string,
) (fixedpoint.Value, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queryCount[asset]++
	if err := m.errs[asset]; err != nil {
		return fixedpoint.Zero, err
	}
	return m.maxBorrowable[asset], nil
}

func (m *mockMarginBorrowRepayService) getQueryCount(asset string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.queryCount[asset]
}

func TestMarginInfoUpdater_GetMaxBorrowable(t *testing.T) {
	svc := newMockMarginBorrowRepayService(map[string]fixedpoint.Value{
		"BTC": fixedpoint.NewFromFloat(1.5),
	})
	config := MarginInfoUpdaterConfig{
		BatchSize: 1,
	}
	config.Defaults()
	updater := NewMarginInfoUpdater(svc, config)

	t.Run("untracked asset is not found", func(t *testing.T) {
		amount, ok := updater.GetMaxBorrowable("BTC")
		assert.False(t, ok, "asset was never added")
		assert.True(t, amount.IsZero())
	})

	t.Run("tracked but not yet refreshed reports zero", func(t *testing.T) {
		updater.AddBorrowableAssets("BTC")

		amount, ok := updater.GetMaxBorrowable("BTC")
		assert.True(t, ok, "asset is tracked once added")
		assert.True(t, amount.IsZero(), "amount defaults to zero before the first Update")
	})

	t.Run("reports the queried amount after Update", func(t *testing.T) {
		updater.Update(context.Background())

		amount, ok := updater.GetMaxBorrowable("BTC")
		assert.True(t, ok)
		assert.Equal(t, "1.5", amount.String())
	})
}

func TestMarginInfoUpdater_Update(t *testing.T) {
	t.Run("updates every asset and emits a callback per asset", func(t *testing.T) {
		values := map[string]fixedpoint.Value{
			"BTC": fixedpoint.NewFromFloat(1),
			"ETH": fixedpoint.NewFromFloat(2),
			"SOL": fixedpoint.NewFromFloat(3),
		}
		config := MarginInfoUpdaterConfig{
			BatchSize: len(values),
		}
		config.Defaults()
		svc := newMockMarginBorrowRepayService(values)
		updater := NewMarginInfoUpdater(svc, config)

		emitted := map[string]fixedpoint.Value{}
		updater.OnMaxBorrowable(func(asset string, amount fixedpoint.Value) {
			emitted[asset] = amount
		})

		for asset := range values {
			updater.AddBorrowableAssets(asset)
		}

		// batch size >= number of assets => all done in one pass
		updater.Update(context.Background())

		for asset, want := range values {
			amount, ok := updater.GetMaxBorrowable(asset)
			assert.True(t, ok)
			assert.Equal(t, want.String(), amount.String(), "GetMaxBorrowable(%s)", asset)
			assert.Equal(t, want.String(), emitted[asset].String(), "emitted(%s)", asset)
		}
		assert.Len(t, emitted, len(values))
	})

	t.Run("more assets than batch size: all updated across cycles", func(t *testing.T) {
		const (
			assetCount = 5
			batchSize  = 2
		)

		values := make(map[string]fixedpoint.Value, assetCount)
		for i := 0; i < assetCount; i++ {
			values[fmt.Sprintf("A%d", i)] = fixedpoint.NewFromFloat(float64(i + 1))
		}

		svc := newMockMarginBorrowRepayService(values)
		config := MarginInfoUpdaterConfig{
			BatchSize: batchSize,
		}
		config.Defaults()
		updater := NewMarginInfoUpdater(svc, config)

		emitCount := map[string]int{}
		updater.OnMaxBorrowable(func(asset string, amount fixedpoint.Value) {
			emitCount[asset]++
		})

		for asset := range values {
			updater.AddBorrowableAssets(asset)
		}

		// A single Update only processes batchSize assets.
		updater.Update(context.Background())
		assert.Len(t, updater.doneAssets, batchSize, "only one batch processed per Update")

		// ceil(5/2) = 3 Update calls are needed to cover every asset. Run the two
		// remaining passes to complete the cycle.
		updater.Update(context.Background())
		updater.Update(context.Background())

		// Every asset must have been refreshed exactly once with the correct value.
		for asset, want := range values {
			amount, ok := updater.GetMaxBorrowable(asset)
			assert.True(t, ok, "GetMaxBorrowable(%s) found", asset)
			assert.Equal(t, want.String(), amount.String(), "GetMaxBorrowable(%s)", asset)
			assert.Equal(t, 1, svc.getQueryCount(asset), "queried once per cycle: %s", asset)
			assert.Equal(t, 1, emitCount[asset], "emitted once per cycle: %s", asset)
		}

		// After the whole set is covered, the cycle resets so the next Update
		// begins refreshing from scratch.
		assert.Empty(t, updater.doneAssets, "doneAssets reset after a full cycle")
		assert.False(t, updater.lastResetDoneTime.IsZero(), "reset time recorded")
	})

	t.Run("second cycle re-queries every asset", func(t *testing.T) {
		values := map[string]fixedpoint.Value{
			"BTC": fixedpoint.NewFromFloat(1),
			"ETH": fixedpoint.NewFromFloat(2),
			"SOL": fixedpoint.NewFromFloat(3),
		}
		svc := newMockMarginBorrowRepayService(values)
		config := MarginInfoUpdaterConfig{
			BatchSize: len(values),
		}
		config.Defaults()
		updater := NewMarginInfoUpdater(svc, config)

		for asset := range values {
			updater.AddBorrowableAssets(asset)
		}

		// first cycle
		updater.Update(context.Background())
		assert.Empty(t, updater.doneAssets, "cycle completes and resets")

		// second cycle: each asset should be queried again
		updater.Update(context.Background())
		for asset := range values {
			assert.Equal(t, 2, svc.getQueryCount(asset), "queried again in the next cycle: %s", asset)
		}
	})

	t.Run("query error keeps the asset pending and blocks the reset", func(t *testing.T) {
		values := map[string]fixedpoint.Value{
			"BTC": fixedpoint.NewFromFloat(1),
			"ETH": fixedpoint.NewFromFloat(2),
		}
		svc := newMockMarginBorrowRepayService(values)
		svc.errs["ETH"] = fmt.Errorf("boom")
		config := MarginInfoUpdaterConfig{
			BatchSize: 10,
		}
		config.Defaults()
		updater := NewMarginInfoUpdater(svc, config)

		emitted := map[string]fixedpoint.Value{}
		updater.OnMaxBorrowable(func(asset string, amount fixedpoint.Value) {
			emitted[asset] = amount
		})

		updater.AddBorrowableAssets("BTC", "ETH")

		// large batch so both are attempted in one pass
		updater.Update(context.Background())

		// BTC succeeded; ETH errored so it is neither recorded as done nor emitted.
		btc, ok := updater.GetMaxBorrowable("BTC")
		assert.True(t, ok)
		assert.Equal(t, "1", btc.String())
		assert.Contains(t, emitted, "BTC")
		assert.NotContains(t, emitted, "ETH")

		// ETH stays pending, so the cycle has not completed and does not reset.
		_, done := updater.doneAssets["ETH"]
		assert.False(t, done, "errored asset is not marked done")
		assert.True(t, updater.lastResetDoneTime.IsZero(), "reset must not happen while an asset is pending")

		// Once the error clears, a subsequent Update finishes the cycle.
		svc.mu.Lock()
		delete(svc.errs, "ETH")
		svc.mu.Unlock()

		updater.Update(context.Background())
		eth, ok := updater.GetMaxBorrowable("ETH")
		assert.True(t, ok)
		assert.Equal(t, "2", eth.String())
		assert.Equal(t, "2", emitted["ETH"].String())
		assert.Empty(t, updater.doneAssets, "cycle resets after the retry succeeds")
	})

	t.Run("no assets is a no-op", func(t *testing.T) {
		svc := newMockMarginBorrowRepayService(nil)
		config := MarginInfoUpdaterConfig{
			BatchSize: 5,
		}
		updater := NewMarginInfoUpdater(svc, config)

		assert.NotPanics(t, func() {
			updater.Update(context.Background())
		})
	})
}
