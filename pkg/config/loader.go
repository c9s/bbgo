package config

import (
	"encoding/json"
	"io/ioutil"
	"reflect"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/bbgo"
)

type SingleExchangeStrategyConfig struct {
	Mounts   []string
	Strategy bbgo.SingleExchangeStrategy
}

type Config struct {
	ExchangeStrategies      []SingleExchangeStrategyConfig
	CrossExchangeStrategies []bbgo.CrossExchangeStrategy
}

type Stash map[string]interface{}

func loadStash(configFile string) (Stash, error) {
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

func Load(configFile string) (*Config, error) {
	var config Config

	stash, err := loadStash(configFile)
	if err != nil {
		return nil, err
	}

	strategies, err := loadExchangeStrategies(stash)
	if err != nil {
		return nil, err
	}

	config.ExchangeStrategies = strategies

	crossExchangeStrategies, err := loadCrossExchangeStrategies(stash)
	if err != nil {
		return nil, err
	}

	config.CrossExchangeStrategies = crossExchangeStrategies

	return &config, nil
}

func loadCrossExchangeStrategies(stash Stash) (strategies []bbgo.CrossExchangeStrategy, err error) {
	exchangeStrategiesConf, ok := stash["crossExchangeStrategies"]
	if !ok {
		return strategies, nil
	}

	configList, ok := exchangeStrategiesConf.([]interface{})
	if !ok {
		return nil, errors.New("expecting list in crossExchangeStrategies")
	}

	for _, entry := range configList {
		configStash, ok := entry.(Stash)
		if !ok {
			return nil, errors.Errorf("strategy config should be a map, given: %T %+v", entry, entry)
		}

		for id, conf := range configStash {
			// look up the real struct type
			if st, ok := bbgo.LoadedExchangeStrategies[id]; ok {
				val, err := reUnmarshal(conf, st)
				if err != nil {
					return nil, err
				}

				strategies = append(strategies, val.(bbgo.CrossExchangeStrategy))
			}
		}

	}

	return strategies, nil
}

func loadExchangeStrategies(stash Stash) (strategies []SingleExchangeStrategyConfig, err error) {
	exchangeStrategiesConf, ok := stash["exchangeStrategies"]
	if !ok {
		return strategies, nil
	}

	configList, ok := exchangeStrategiesConf.([]interface{})
	if !ok {
		return nil, errors.New("expecting list in exchangeStrategies")
	}

	for _, entry := range configList {
		configStash, ok := entry.(Stash)
		if !ok {
			return nil, errors.Errorf("strategy config should be a map, given: %T %+v", entry, entry)
		}

		var mounts []string
		if val, ok := configStash["on"]; ok {
			if values, ok := val.([]string); ok {
				mounts = append(mounts, values...)
			} else if str, ok := val.(string); ok {
				mounts = append(mounts, str)
			}
		}

		for id, conf := range configStash {
			// look up the real struct type
			if st, ok := bbgo.LoadedExchangeStrategies[id]; ok {
				val, err := reUnmarshal(conf, st)
				if err != nil {
					return nil, err
				}

				strategies = append(strategies, SingleExchangeStrategyConfig{
					Mounts:   mounts,
					Strategy: val.(bbgo.SingleExchangeStrategy),
				})
			}
		}
	}

	return strategies, nil
}

func reUnmarshal(conf interface{}, tpe interface{}) (interface{}, error) {
	// get the type "*Strategy"
	rt := reflect.TypeOf(tpe)

	// allocate new object from the given type
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

	return val.Elem().Interface(), nil
}
