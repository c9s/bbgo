package cmdutil

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/types"
)

func NewExchangeWithEnvVarPrefix(n types.ExchangeName, varPrefix string) (types.Exchange, error) {
	if len(varPrefix) == 0 {
		varPrefix = n.String()
	}

	switch n {

	case types.ExchangeBinance:
		key := viper.GetString(varPrefix + "-api-key")
		secret := viper.GetString(varPrefix + "-api-secret")
		if len(key) == 0 || len(secret) == 0 {
			return nil, errors.New("binance: empty key or secret")
		}

		return binance.New(key, secret), nil

	case types.ExchangeMax:
		key := viper.GetString(varPrefix + "-api-key")
		secret := viper.GetString(varPrefix + "-api-secret")
		if len(key) == 0 || len(secret) == 0 {
			return nil, errors.New("max: empty key or secret")
		}

		return max.New(key, secret), nil

	default:
		return nil, errors.Errorf("unsupported exchange: %v", n)

	}
}

// NewExchange constructor exchange object from viper config.
func NewExchange(n types.ExchangeName) (types.Exchange, error) {
	return NewExchangeWithEnvVarPrefix(n, "")
}
