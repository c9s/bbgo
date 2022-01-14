package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

type DataFetcher func() (interface{}, error)

const cacheExpiry = 24 * time.Hour

// WithCache let you use the cache with the given cache key, variable reference and your data fetcher,
// The key must be an unique ID.
// obj is the pointer of your local variable
// fetcher is the closure that will fetch your remote data or some slow operation.
func WithCache(key string, obj interface{}, fetcher DataFetcher) error {
	cacheDir := CacheDir()
	cacheFile := path.Join(cacheDir, key+".json")

	stat, err := os.Stat(cacheFile)
	if os.IsNotExist(err) || (stat != nil && time.Since(stat.ModTime()) > cacheExpiry) {
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

func LoadExchangeMarketsWithCache(ctx context.Context, ex types.Exchange) (markets types.MarketMap, err error) {
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
