package loader

import (
	"encoding/json"
	"io/ioutil"
	"reflect"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func LoadExchangeStrategies(configFile string) (strategies []bbgo.SingleExchangeStrategy, err error) {
	config, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	conf := make(map[string]interface{})
	if err := yaml.Unmarshal(config, conf); err != nil {
		return nil, err
	}

	exchangeStrategiesConf, ok := conf["exchangeStrategies"]
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
			if st, ok := bbgo.LoadedStrategies[id]; ok {
				// get the type "*Strategy"
				rt := reflect.TypeOf(st)
				val := reflect.New(rt)

				// now we have &(*Strategy) -> **Strategy
				valinf := val.Interface()

				plain, err := json.Marshal(conf)
				if err != nil {
					return nil, err
				}

				if err := json.Unmarshal(plain, valinf); err != nil {
					return nil, err
				}

				strategies = append(strategies, val.Elem().Interface().(bbgo.SingleExchangeStrategy))
			}
		}
	}

	return strategies, nil
}
