package xmaker

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/bbgo/sessionworker"
	"github.com/c9s/bbgo/pkg/types"
)

const borrowableAssetsWorkerID = "borrowable-assets"
const borrowableAssetsUpdateInterval = 5 * time.Minute

var logger = logrus.WithFields(logrus.Fields{
	"strategy": ID,
	"worker":   borrowableAssetsWorkerID,
})

type BorrowableAssetResult struct {
	BaseBorrowable  bool
	QuoteBorrowable bool
}

type AssetsBorrowablity map[string]bool

// BorrowableAssetsWorker periodically queries the margin asset catalog of the
// hedge exchange to learn whether the hedge market's base/quote currencies can
// currently be borrowed. The cached result is consumed by updateQuote to gate
// the maker side that would require borrowing a non-borrowable asset.
type BorrowableAssetsWorker struct {
	mu sync.Mutex

	assets  map[string]struct{}
	running bool
}

func createBorrowableAssetsWorker(ctx context.Context, session *bbgo.ExchangeSession, market types.Market) *BorrowableAssetsWorker {
	handle := sessionworker.Get(session, borrowableAssetsWorkerID)
	if handle == nil {
		handle = sessionworker.Start(ctx, session, borrowableAssetsWorkerID, &BorrowableAssetsWorker{
			assets: make(map[string]struct{}),
		})
	}
	worker := handle.Worker().(*BorrowableAssetsWorker)
	worker.AddAsset(market.BaseCurrency)
	worker.AddAsset(market.QuoteCurrency)
	return worker
}

func (w *BorrowableAssetsWorker) AddAsset(asset string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.assets[asset] = struct{}{}
}

func (w *BorrowableAssetsWorker) fetch(
	ctx context.Context, service types.MarginAssetQueryService,
) AssetsBorrowablity {
	var assetList []string
	for asset := range w.assets {
		assetList = append(assetList, asset)
	}
	assets, err := service.QueryMarginAssets(ctx, assetList...)
	if err != nil {
		logger.WithError(err).Warnf("unable to query margin assets: %v", assetList)
		return nil
	}

	// default to not borrowable; only flip to true when the exchange reports so.
	results := make(map[string]bool, len(assets))
	for _, asset := range assets {
		results[asset.Asset] = asset.IsBorrowable
	}

	return AssetsBorrowablity(results)
}

func (w *BorrowableAssetsWorker) Run(ctx context.Context, handle *sessionworker.Handle) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return
	}

	w.running = true
	go w.run(ctx, handle)
}

func (w *BorrowableAssetsWorker) run(ctx context.Context, handle *sessionworker.Handle) {
	defer logger.Info("borrowable assets worker exited")

	session := handle.Session()

	service, ok := session.Exchange.(types.MarginAssetQueryService)
	if !ok {
		// The exchange does not expose a margin asset catalog; we cannot gate on
		// borrowability, so treat both currencies as borrowable and stop.
		logger.Warnf(
			"exchange %s does not implement MarginAssetQueryService", session.Name,
		)
		return
	}

	// warm the cache immediately before the first tick.
	if rst := w.fetch(ctx, service); rst != nil {
		handle.SetValue(rst)
	}

	ticker := time.NewTicker(borrowableAssetsUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if rst := w.fetch(ctx, service); rst != nil {
				handle.SetValue(rst)
			}
		}
	}
}

// getBorrowableAssetResult reads the cached borrowability result for the given hedge
// market, or nil if the worker has not produced a value yet.
// IMPORTANT: we assume the passed session is a margin session, the caller should check this before calling this function.
func (s *Strategy) getBorrowableAssetResult(session *bbgo.ExchangeSession, market types.Market) *BorrowableAssetResult {
	if !session.Margin {
		s.logger.Warnf("getBorrowableAssetResult called on non-margin session %s, returning nil", session.Name)
		return nil
	}
	workerHandle := sessionworker.Get(session, borrowableAssetsWorkerID)
	if workerHandle == nil {
		return nil
	}

	val := workerHandle.Value()
	if val == nil {
		return nil
	}

	rst, ok := val.(AssetsBorrowablity)
	if !ok {
		s.logger.Warnf("unable to assert borrowable assets result from session: %s", session.Name)
		return nil
	}
	baseBorrowable, baseFound := rst[market.BaseCurrency]
	quoteBorrowable, quoteFound := rst[market.QuoteCurrency]
	if !baseFound || !quoteFound {
		// return nil to disable the borrowablity check.
		s.logger.Warnf("at least one of the assets is missing from the borrowable assets result on %s: %s", session.Name, market.Symbol)
		return nil
	}

	return &BorrowableAssetResult{
		BaseBorrowable:  baseBorrowable,
		QuoteBorrowable: quoteBorrowable,
	}
}
