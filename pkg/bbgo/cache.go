package bbgo

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"reflect"

	"github.com/pkg/errors"
)

func WithCache(file string, obj interface{}, do func() (interface{}, error)) error {
	cacheDir := CacheDir()
	cacheFile := path.Join(cacheDir, file)

	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		data, err := do()
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
