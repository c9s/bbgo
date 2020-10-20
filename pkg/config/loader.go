package config

import (
	"encoding/json"
	"io/ioutil"
	"reflect"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/bbgo"
)

type Config struct {
	ExchangeStrategies      []bbgo.SingleExchangeStrategy
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
	return &config, nil
}

func loadExchangeStrategies(stash Stash) (strategies []bbgo.SingleExchangeStrategy, err error) {
	exchangeStrategiesConf, ok := stash["exchangeStrategies"]
	if !ok {
		return nil, errors.New("exchangeStrategies is not defined")
	}

	list, ok := exchangeStrategiesConf.([]interface{})
	if !ok {
		return nil, errors.New("exchangeStrategies should be a list")
	}

	for _, entry := range list {
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

				strategies = append(strategies, val.(bbgo.SingleExchangeStrategy))
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
