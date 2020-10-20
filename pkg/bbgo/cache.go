package bbgo

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"reflect"

	"github.com/pkg/errors"
)


type DataFetcher func() (interface{}, error)

// WithCache let you use the cache with the given cache key, variable reference and your data fetcher,
// The key must be an unique ID.
// obj is the pointer of your local variable
// fetcher is the closure that will fetch your remote data or some slow operation.
func WithCache(key string, obj interface{}, fetcher DataFetcher) error {
	cacheDir := CacheDir()
	cacheFile := path.Join(cacheDir, key+ ".json")

	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
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
