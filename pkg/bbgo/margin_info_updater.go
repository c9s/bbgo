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

	doneAssets        map[string]struct{}
	lastResetDoneTime time.Time

	maxBorrowableCallbacks []MaxBorrowableCallback

	mu sync.Mutex
}

func NewMarginInfoUpdater(
	service types.MarginBorrowRepayService,
) *MarginInfoUpdater {
	return &MarginInfoUpdater{
		assets:     make(map[string]fixedpoint.Value),
		doneAssets: make(map[string]struct{}),
		service:    service,
	}
}

// Run starts the update workers for the given interval.
func (m *MarginInfoUpdater) Run(
	ctx context.Context,
	interval types.Duration,
	batchSize int,
	coolDown time.Duration,
) {
	ticker := time.NewTicker(timejitter.Milliseconds(interval.Duration(), 500))
	defer ticker.Stop()

	m.Update(ctx, batchSize)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if !m.lastResetDoneTime.IsZero() && now.Sub(m.lastResetDoneTime) <= coolDown {
				// still in cooldown period, skip this update cycle
				continue
			}
			m.Update(ctx, batchSize)
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
	batchSize int,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending := make([]string, 0, len(m.assets))
	for asset := range m.assets {
		if _, done := m.doneAssets[asset]; !done {
			pending = append(pending, asset)
		}
	}

	if batchSize > len(pending) {
		batchSize = len(pending)
	}

	for _, asset := range pending[:batchSize] {
		maxBorrowable, err := m.service.QueryMarginAssetMaxBorrowable(
			ctx,
			asset,
		)

		if err != nil {
			log.WithError(err).Errorf("query margin asset max borrowable error: %s", asset)
			continue
		}

		m.assets[asset] = maxBorrowable
		m.doneAssets[asset] = struct{}{}

		m.EmitMaxBorrowable(asset, maxBorrowable)
	}

	// once every asset has been refreshed, reset to begin a new cycle.
	if len(m.doneAssets) >= len(m.assets) {
		m.doneAssets = make(map[string]struct{})
		m.lastResetDoneTime = time.Now()
	}
}
