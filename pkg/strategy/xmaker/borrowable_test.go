package xmaker

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type mockMarginAssetQueryService struct {
	assets []types.MarginBorrowableAsset
	err    error
}

func (m *mockMarginAssetQueryService) QueryMarginAssets(
	_ context.Context, assets ...string,
) ([]types.MarginBorrowableAsset, error) {
	if m.err != nil {
		return nil, m.err
	}

	if len(assets) == 0 {
		return m.assets, nil
	}

	filter := make(map[string]struct{}, len(assets))
	for _, a := range assets {
		filter[a] = struct{}{}
	}

	var out []types.MarginBorrowableAsset
	for _, a := range m.assets {
		if _, ok := filter[a.Asset]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

func newTestBorrowableAssetsWorker(assets ...string) *BorrowableAssetsWorker {
	w := &BorrowableAssetsWorker{assets: make(map[string]struct{})}
	for _, asset := range assets {
		w.AddAsset(asset)
	}
	return w
}

func TestBorrowableAssetsWorker_update(t *testing.T) {
	ctx := context.Background()

	t.Run("both borrowable", func(t *testing.T) {
		w := newTestBorrowableAssetsWorker("BTC", "USDT")
		svc := &mockMarginAssetQueryService{assets: []types.MarginBorrowableAsset{
			{Asset: "BTC", IsBorrowable: true},
			{Asset: "USDT", IsBorrowable: true},
		}}

		rst := w.fetch(ctx, svc)
		if assert.NotNil(t, rst) {
			assert.True(t, rst["BTC"])
			assert.True(t, rst["USDT"])
		}
	})

	t.Run("base not borrowable", func(t *testing.T) {
		w := newTestBorrowableAssetsWorker("BTC", "USDT")
		svc := &mockMarginAssetQueryService{assets: []types.MarginBorrowableAsset{
			{Asset: "BTC", IsBorrowable: false},
			{Asset: "USDT", IsBorrowable: true},
		}}

		rst := w.fetch(ctx, svc)
		if assert.NotNil(t, rst) {
			assert.False(t, rst["BTC"])
			assert.True(t, rst["USDT"])
		}
	})

	t.Run("quote not borrowable", func(t *testing.T) {
		w := newTestBorrowableAssetsWorker("BTC", "USDT")
		svc := &mockMarginAssetQueryService{assets: []types.MarginBorrowableAsset{
			{Asset: "BTC", IsBorrowable: true},
			{Asset: "USDT", IsBorrowable: false},
		}}

		rst := w.fetch(ctx, svc)
		if assert.NotNil(t, rst) {
			assert.True(t, rst["BTC"])
			assert.False(t, rst["USDT"])
		}
	})

	t.Run("missing asset defaults to not borrowable", func(t *testing.T) {
		w := newTestBorrowableAssetsWorker("BTC", "USDT")
		// only base is returned; quote is absent from the catalog response.
		svc := &mockMarginAssetQueryService{assets: []types.MarginBorrowableAsset{
			{Asset: "BTC", IsBorrowable: true},
		}}

		rst := w.fetch(ctx, svc)
		if assert.NotNil(t, rst) {
			assert.True(t, rst["BTC"])
			assert.False(t, rst["USDT"])
		}
	})

	t.Run("query error returns nil", func(t *testing.T) {
		w := newTestBorrowableAssetsWorker("BTC", "USDT")
		svc := &mockMarginAssetQueryService{err: assert.AnError}

		rst := w.fetch(ctx, svc)
		assert.Nil(t, rst)
	})
}

func TestStrategy_getBorrowableAssets_NoWorker(t *testing.T) {
	s := &Strategy{logger: logrus.New()}
	session := &bbgo.ExchangeSession{ExchangeSessionConfig: bbgo.ExchangeSessionConfig{Margin: true}}
	// no worker registered for this session -> nil, no panic.
	assert.Nil(t, s.getBorrowableAssetResult(session, types.Market{}))
}

func TestStrategy_getBorrowableAssets_NonMarginSession(t *testing.T) {
	s := &Strategy{logger: logrus.New()}
	session := &bbgo.ExchangeSession{ExchangeSessionConfig: bbgo.ExchangeSessionConfig{Margin: false}}
	assert.Nil(t, s.getBorrowableAssetResult(session, types.Market{}))
}
