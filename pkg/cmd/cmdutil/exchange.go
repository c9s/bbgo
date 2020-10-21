package cmdutil

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/types"
)

// NewExchange constructor exchange object from viper config.
func NewExchange(n types.ExchangeName) (types.Exchange, error) {
	switch n {

	case types.ExchangeBinance:
		key := viper.GetString("binance-api-key")
		secret := viper.GetString("binance-api-secret")
		if len(key) == 0 || len(secret) == 0 {
			return nil, errors.New("binance: empty key or secret")
		}

		return binance.New(key, secret), nil

	case types.ExchangeMax:
		key := viper.GetString("max-api-key")
		secret := viper.GetString("max-api-secret")
		if len(key) == 0 || len(secret) == 0 {
			return nil, errors.New("max: empty key or secret")
		}

		return max.New(key, secret), nil

	}

	return nil, nil
}
