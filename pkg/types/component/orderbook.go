package component

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

func StreamBookHealthCheck(ctx context.Context, checkInterval, reconnectThreshold time.Duration) func(*types.StreamOrderBook) {
	return func(book *types.StreamOrderBook) {
		go func() {
			ticker := time.NewTicker(checkInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case _, ok := <-ticker.C:
					if !ok {
						return
					}
					lastUpdate := book.LastUpdateTime()
					if lastUpdate.IsZero() {
						continue
					}
					if book.Stream == nil {
						continue
					}
					if time.Since(lastUpdate) >= reconnectThreshold {
						book.Stream.Reconnect()
					}
				}
			}
		}()
	}
}
