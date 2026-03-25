package component

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

func StreamBookHealthCheck(ctx context.Context, checkInterval, reconnectThreshold time.Duration) func(*types.StreamOrderBook) {
	return func(book *types.StreamOrderBook) {
		logger := logrus.WithFields(logrus.Fields{
			"component": "healthcheck",
			"symbol":    book.Symbol,
		})
		go func() {
			ticker := time.NewTicker(checkInterval)
			defer ticker.Stop()

			logger.Infof(
				"starting stream book health check %s: %s (%s)",
				book.Symbol, checkInterval.String(), reconnectThreshold.String(),
			)

			for {
				select {
				case <-ctx.Done():
					logger.Info("stream book health check context done")
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
					duration := time.Since(lastUpdate)
					if duration >= reconnectThreshold {
						logger.Warnf(
							"last update time exceeds threshold, reconnecting stream: %s >= %s",
							duration.String(),
							reconnectThreshold.String(),
						)
						book.Stream.Reconnect()
					}
				}
			}
		}()
	}
}
