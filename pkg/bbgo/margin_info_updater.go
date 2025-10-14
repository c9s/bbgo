package bbgo

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util/timejitter"
)

type MaxBorrowableCallback func(asset string, amount fixedpoint.Value)

//go:generate callbackgen -type MarginInfoUpdater
type MarginInfoUpdater struct {
	service types.MarginBorrowRepayService

	assets map[string]fixedpoint.Value

	maxBorrowableCallbacks []MaxBorrowableCallback

	mu sync.Mutex
}

func NewMarginInfoUpdater(
	service types.MarginBorrowRepayService,
) *MarginInfoUpdater {
	return &MarginInfoUpdater{
		assets:  make(map[string]fixedpoint.Value),
		service: service,
	}
}

// Run starts the update workers for the given interval.
func (m *MarginInfoUpdater) Run(
	ctx context.Context,
	interval types.Duration,
) {
	ticker := time.NewTicker(timejitter.Milliseconds(interval.Duration(), 500))
	defer ticker.Stop()

	m.Update(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Update(ctx)
		}
	}
}

// AddBorrowableAssets adds the assets to the updater.
func (m *MarginInfoUpdater) AddBorrowableAssets(
	assets ...string,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, asset := range assets {
		m.assets[asset] = fixedpoint.Zero
	}
}

func (m *MarginInfoUpdater) GetMaxBorrowable(asset string) (fixedpoint.Value, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	amount, ok := m.assets[asset]
	return amount, ok
}

// Update queries the max borrowable amount for each asset and emit update events.
func (m *MarginInfoUpdater) Update(
	ctx context.Context,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for asset := range m.assets {
		maxBorrowable, err := m.service.QueryMarginAssetMaxBorrowable(
			ctx,
			asset,
		)

		if err != nil {
			log.WithError(err).Errorf("query margin asset max borrowable error: %s", asset)
			continue
		}

		m.assets[asset] = maxBorrowable

		m.EmitMaxBorrowable(asset, maxBorrowable)
	}
}
