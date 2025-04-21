package bbgo

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util/timejitter"
)

type MaxBorrowableCallback func(asset string, amount fixedpoint.Value)

//go:generate callbackgen -type MarginInfoUpdater
type MarginInfoUpdater struct {
	BorrowRepayService types.MarginBorrowRepayService

	assets                 map[string]struct{}
	maxBorrowableCallbacks []MaxBorrowableCallback
}

func NewMarginInfoUpdaterFromExchange(
	exchange types.Exchange,
) MarginInfoUpdater {
	m := MarginInfoUpdater{}
	if s, ok := exchange.(types.MarginBorrowRepayService); ok {
		m.BorrowRepayService = s
	}
	return m
}

// Run starts the update workers for the given interval.
func (m *MarginInfoUpdater) Run(
	ctx context.Context,
	interval types.Duration,
) {
	ticker := time.NewTicker(timejitter.Milliseconds(interval.Duration(), 500))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.UpdateMaxBorrowable(ctx)
		}
	}
}

// AddAssets adds the assets to the updater.
func (m *MarginInfoUpdater) AddAssets(
	assets ...string,
) {
	if m.assets == nil {
		m.assets = make(map[string]struct{})
	}
	for _, asset := range assets {
		m.assets[asset] = struct{}{}
	}
}

// UpdateMaxBorrowable queries the max borrowable amount for each asset and emit update events.
func (m *MarginInfoUpdater) UpdateMaxBorrowable(
	ctx context.Context,
) (failedAssets []string) {
	if m.BorrowRepayService == nil {
		return
	}
	for asset := range m.assets {
		maxBorrable, err := m.BorrowRepayService.QueryMarginAssetMaxBorrowable(
			ctx,
			asset,
		)
		if err != nil {
			failedAssets = append(failedAssets, asset)
			continue
		}
		m.EmitMaxBorrowable(asset, maxBorrable)
	}
	return
}
