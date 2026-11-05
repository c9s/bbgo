package autoborrow

import (
	"context"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

const marginAssetInfoRetryInterval = time.Minute

type marginAssetInfoCache struct {
	mu           sync.Mutex
	infos        map[string]types.MarginAssetInfo // nil after a failed bulk fetch
	refreshAfter time.Time
}

func (c *marginAssetInfoCache) get(
	ctx context.Context, service types.MarginAssetInfoService, assets []string, asset string,
	now time.Time, ttl time.Duration,
) (types.MarginAssetInfo, error) {
	c.mu.Lock()
	if !now.Before(c.refreshAfter) {
		infos, err := service.QueryMarginAssetInfos(ctx, assets)
		if err != nil {
			log.WithError(err).Warn("bulk margin asset information query failed, falling back to per-asset queries")
			c.infos = nil
			c.refreshAfter = now.Add(marginAssetInfoRetryInterval)
		} else {
			c.infos = infos
			c.refreshAfter = now.Add(ttl)
		}
	}
	info, ok := c.infos[asset]
	c.mu.Unlock()

	if ok {
		return info, nil
	}
	return service.QueryMarginAssetInfo(ctx, asset)
}
