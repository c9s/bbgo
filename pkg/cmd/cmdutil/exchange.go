package cmdutil

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/types"
)

func NewExchangeStandard(n types.ExchangeName, key, secret string) (types.Exchange, error) {
	if len(key) == 0 || len(secret) == 0 {
		return nil, errors.New("binance: empty key or secret")
	}

	switch n {

	case types.ExchangeBinance:
		return binance.New(key, secret), nil

	case types.ExchangeMax:
		return max.New(key, secret), nil

	default:
		return nil, fmt.Errorf("unsupported exchange: %v", n)

	}
}

func NewExchangeWithEnvVarPrefix(n types.ExchangeName, varPrefix string) (types.Exchange, error) {
	if len(varPrefix) == 0 {
		varPrefix = n.String()
	}

	varPrefix = strings.ToUpper(varPrefix)

	key := os.Getenv(varPrefix + "_API_KEY")
	secret := os.Getenv(varPrefix + "_API_SECRET")
	if len(key) == 0 || len(secret) == 0 {
		return nil, fmt.Errorf("%s: empty key or secret, env var prefix: %s", n, varPrefix)
	}

	return NewExchangeStandard(n, key, secret)
}

// NewExchange constructor exchange object from viper config.
func NewExchange(n types.ExchangeName) (types.Exchange, error) {
	return NewExchangeWithEnvVarPrefix(n, "")
}
