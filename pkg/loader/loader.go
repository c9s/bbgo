package loader

import (
	"encoding/json"
	"io/ioutil"
	"reflect"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/bbgo"
)

type Stash map[string]interface{}

func Load(configFile string) (Stash, error) {
	config, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	stash := make(Stash)
	if err := yaml.Unmarshal(config, stash); err != nil {
		return nil, err
	}

	return stash, err
}

func LoadExchangeStrategies(configFile string) (strategies []bbgo.SingleExchangeStrategy, err error) {
	stash, err := Load(configFile)
	if err != nil {
		return nil, err
	}

	exchangeStrategiesConf, ok := stash["exchangeStrategies"]
	if !ok {
		return nil, errors.New("exchangeStrategies is not defined")
	}

	strategiesConfList, ok := exchangeStrategiesConf.([]interface{})
	if !ok {
		return nil, errors.New("exchangeStrategies should be a list")
	}

	for _, strategiesConf := range strategiesConfList {
		sConf, ok := strategiesConf.(map[string]interface{})
		if !ok {
			return nil, errors.New("strategy config should be a map")
		}

		for id, conf := range sConf {
			if st, ok := bbgo.LoadedExchangeStrategies[id]; ok {
				// get the type "*Strategy"
				rt := reflect.TypeOf(st)
				val := reflect.New(rt)

				// now we have &(*Strategy) -> **Strategy
				valRef := val.Interface()

				plain, err := json.Marshal(conf)
				if err != nil {
					return nil, err
				}

				if err := json.Unmarshal(plain, valRef); err != nil {
					return nil, errors.Wrapf(err, "json parsing error, given payload: %s", plain)
				}

				strategies = append(strategies, val.Elem().Interface().(bbgo.SingleExchangeStrategy))
			}
		}
	}

	return strategies, nil
}
