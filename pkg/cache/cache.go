package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/util/backoff"
)

const memCacheExpiry = 5 * time.Minute
const fileCacheExpiry = 24 * time.Hour

var globalMarketMemCache *marketMemCache = newMarketMemCache()

type marketMemCache struct {
	sync.Mutex
	markets map[string]marketMapWithTime
}

type marketMapWithTime struct {
	updatedAt time.Time
	markets   types.MarketMap
}

func newMarketMemCache() *marketMemCache {
	cache := &marketMemCache{
		markets: make(map[string]marketMapWithTime),
	}
	return cache
}

func (c *marketMemCache) IsOutdated(exName string) bool {
	c.Lock()
	defer c.Unlock()

	data, ok := c.markets[exName]
	return !ok || time.Since(data.updatedAt) > memCacheExpiry
}

func (c *marketMemCache) Set(exName string, markets types.MarketMap) {
	c.Lock()
	defer c.Unlock()

	c.markets[exName] = marketMapWithTime{
		updatedAt: time.Now(),
		markets:   markets,
	}
}

func (c *marketMemCache) Get(exName string) (types.MarketMap, bool) {
	c.Lock()
	defer c.Unlock()

	markets, ok := c.markets[exName]
	if !ok {
		return nil, false
	}

	copied := types.MarketMap{}
	for key, val := range markets.markets {
		copied[key] = val
	}
	return copied, true
}

type DataFetcher func() (interface{}, error)

// WithCache let you use the cache with the given cache key, variable reference and your data fetcher,
// The key must be an unique ID.
// obj is the pointer of your local variable
// fetcher is the closure that will fetch your remote data or some slow operation.
func WithCache(key string, obj interface{}, fetcher DataFetcher) error {
	cacheDir := CacheDir()
	cacheFile := path.Join(cacheDir, key+".json")

	stat, err := os.Stat(cacheFile)
	if os.IsNotExist(err) || (stat != nil && time.Since(stat.ModTime()) > fileCacheExpiry) {
		log.Debugf("cache %s not found or cache expired, executing fetcher callback to get the data", cacheFile)

		data, err := fetcher()
		if err != nil {
			return err
		}

		out, err := json.Marshal(data)
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(cacheFile, out, 0666); err != nil {
			return err
		}

		rv := reflect.ValueOf(obj).Elem()
		if !rv.CanSet() {
			return errors.New("can not set cache object value")
		}

		rv.Set(reflect.ValueOf(data))

	} else {
		log.Debugf("cache %s found", cacheFile)

		data, err := ioutil.ReadFile(cacheFile)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(data, obj); err != nil {
			return err
		}
	}

	return nil
}

func LoadExchangeMarketsWithCache(ctx context.Context, ex types.ExchangePublic) (markets types.MarketMap, err error) {
	inMem, ok := util.GetEnvVarBool("USE_MARKETS_CACHE_IN_MEMORY")
	if ok && inMem {
		return loadMarketsFromMem(ctx, ex)
	}

	// fallback to use files as cache
	return loadMarketsFromFile(ctx, ex)
}

// loadMarketsFromMem is useful for one process to run multiple bbgos in different go routines.
func loadMarketsFromMem(ctx context.Context, ex types.ExchangePublic) (markets types.MarketMap, _ error) {
	exName := ex.Name().String()
	if globalMarketMemCache.IsOutdated(exName) {
		op := func() error {
			rst, err2 := ex.QueryMarkets(ctx)
			if err2 != nil {
				return err2
			}

			markets = rst
			globalMarketMemCache.Set(exName, rst)
			return nil
		}

		if err := backoff.RetryGeneral(ctx, op); err != nil {
			return nil, err
		}

		return markets, nil
	}

	rst, _ := globalMarketMemCache.Get(exName)
	return rst, nil
}

func loadMarketsFromFile(ctx context.Context, ex types.ExchangePublic) (markets types.MarketMap, err error) {
	key := fmt.Sprintf("%s-markets", ex.Name())
	if futureExchange, implemented := ex.(types.FuturesExchange); implemented {
		settings := futureExchange.GetFuturesSettings()
		if settings.IsFutures {
			key = fmt.Sprintf("%s-futures-markets", ex.Name())
		}
	}

	err = WithCache(key, &markets, func() (interface{}, error) {
		return ex.QueryMarkets(ctx)
	})
	return markets, err
}
