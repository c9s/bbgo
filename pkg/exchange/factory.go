package exchange

import (
	"fmt"
	"os"
	"strings"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/bitget"
	"github.com/c9s/bbgo/pkg/exchange/bybit"
	"github.com/c9s/bbgo/pkg/exchange/kucoin"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/exchange/okex"
	"github.com/c9s/bbgo/pkg/types"
)

func NewPublic(exchangeName types.ExchangeName) (types.Exchange, error) {
	exMinimal, err := New(exchangeName, "", "", "")
	if err != nil {
		return nil, err
	}

	if ex, ok := exMinimal.(types.Exchange); ok {
		return ex, nil
	}

	return nil, fmt.Errorf("exchange %T does not implement types.Exchange", exMinimal)
}

func New(n types.ExchangeName, key, secret, passphrase string) (types.ExchangeMinimal, error) {
	switch n {

	case types.ExchangeBinance:
		return binance.New(key, secret), nil

	case types.ExchangeMax:
		return max.New(key, secret), nil

	case types.ExchangeOKEx:
		return okex.New(key, secret, passphrase), nil

	case types.ExchangeKucoin:
		return kucoin.New(key, secret, passphrase), nil

	case types.ExchangeBitget:
		return bitget.New(key, secret, passphrase), nil

	case types.ExchangeBybit:
		return bybit.New(key, secret)

	default:
		return nil, fmt.Errorf("unsupported exchange: %v", n)

	}
}

// NewWithEnvVarPrefix allocate and initialize the exchange instance with the given environment variable prefix
// When the varPrefix is a empty string, the default exchange name will be used as the prefix
func NewWithEnvVarPrefix(n types.ExchangeName, varPrefix string) (types.ExchangeMinimal, error) {
	if len(varPrefix) == 0 {
		varPrefix = n.String()
	}

	varPrefix = strings.ToUpper(varPrefix)

	key := os.Getenv(varPrefix + "_API_KEY")
	secret := os.Getenv(varPrefix + "_API_SECRET")
	if len(key) == 0 || len(secret) == 0 {
		return nil, fmt.Errorf("can not initialize exchange %s: empty key or secret, env var prefix: %s", n, varPrefix)
	}

	passphrase := os.Getenv(varPrefix + "_API_PASSPHRASE")
	return New(n, key, secret, passphrase)
}
