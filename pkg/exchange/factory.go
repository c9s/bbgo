package exchange

import (
	"fmt"
	"os"
	"strings"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/ftx"
	"github.com/c9s/bbgo/pkg/exchange/kucoin"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/exchange/okex"
	"github.com/c9s/bbgo/pkg/types"
)

func NewPublic(exchangeName types.ExchangeName) (types.Exchange, error) {
	return NewStandard(exchangeName, "", "", "", "")
}

func NewStandard(n types.ExchangeName, key, secret, passphrase, subAccount string) (types.Exchange, error) {
	switch n {

	case types.ExchangeFTX:
		return ftx.NewExchange(key, secret, subAccount), nil

	case types.ExchangeBinance:
		return binance.New(key, secret), nil

	case types.ExchangeMax:
		return max.New(key, secret), nil

	case types.ExchangeOKEx:
		return okex.New(key, secret, passphrase), nil

	case types.ExchangeKucoin:
		return kucoin.New(key, secret, passphrase), nil

	default:
		return nil, fmt.Errorf("unsupported exchange: %v", n)

	}
}

func NewWithEnvVarPrefix(n types.ExchangeName, varPrefix string) (types.Exchange, error) {
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
	subAccount := os.Getenv(varPrefix + "_SUBACCOUNT")
	return NewStandard(n, key, secret, passphrase, subAccount)
}

// New constructor exchange object from viper config.
func New(n types.ExchangeName) (types.Exchange, error) {
	return NewWithEnvVarPrefix(n, "")
}
