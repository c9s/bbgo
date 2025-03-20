package exchange

import (
	"fmt"
	"os"
	"strings"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/bitget"
	"github.com/c9s/bbgo/pkg/exchange/bybit"
	"github.com/c9s/bbgo/pkg/exchange/coinbase"
	"github.com/c9s/bbgo/pkg/exchange/kucoin"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/exchange/okex"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	ExchangeOptionsKeyAPIKey        = "API_KEY"
	ExchangeOptionsKeyAPISecret     = "API_SECRET"
	ExchangeOptionsKeyAPIPassphrase = "API_PASSPHRASE"
)

// ExchangeOptions is a map of exchange options used to initialize an exchange
type ExchangeOptions map[string]string

// ExchangeEnvLoader is a function type to load exchange options from environment variables
// According to pkg/bbgo/environment.go, we must to get the environment variable with %s-api-key so that we can add this custom exchange.
type ExchangeEnvLoader func(varPrefix string) (ExchangeOptions, error)

// ExchangeConstructor is a function type to create an exchange instance with the given options
type ExchangeConstructor func(ExchangeOptions) (types.Exchange, error)

type ExchangeFactory struct {
	EnvLoader   ExchangeEnvLoader
	Constructor ExchangeConstructor
}

var exchangeFactories = map[types.ExchangeName]ExchangeFactory{
	types.ExchangeBinance: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options ExchangeOptions) (types.Exchange, error) {
			return binance.New(options[ExchangeOptionsKeyAPIKey], options[ExchangeOptionsKeyAPISecret]), nil
		},
	},
	types.ExchangeMax: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options ExchangeOptions) (types.Exchange, error) {
			return max.New(options[ExchangeOptionsKeyAPIKey], options[ExchangeOptionsKeyAPISecret]), nil
		},
	},
	types.ExchangeOKEx: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options ExchangeOptions) (types.Exchange, error) {
			return okex.New(options[ExchangeOptionsKeyAPIKey], options[ExchangeOptionsKeyAPISecret], options[ExchangeOptionsKeyAPIPassphrase]), nil
		},
	},
	types.ExchangeKucoin: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options ExchangeOptions) (types.Exchange, error) {
			return kucoin.New(options[ExchangeOptionsKeyAPIKey], options[ExchangeOptionsKeyAPISecret], options[ExchangeOptionsKeyAPIPassphrase]), nil
		},
	},
	types.ExchangeBitget: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options ExchangeOptions) (types.Exchange, error) {
			return bitget.New(options[ExchangeOptionsKeyAPIKey], options[ExchangeOptionsKeyAPISecret], options[ExchangeOptionsKeyAPIPassphrase]), nil
		},
	},
	types.ExchangeBybit: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options ExchangeOptions) (types.Exchange, error) {
			return bybit.New(options[ExchangeOptionsKeyAPIKey], options[ExchangeOptionsKeyAPISecret])
		},
	},
	types.ExchangeCoinBase: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options ExchangeOptions) (types.Exchange, error) {
			return coinbase.New(options[ExchangeOptionsKeyAPIKey], options[ExchangeOptionsKeyAPISecret], options[ExchangeOptionsKeyAPIPassphrase], 0), nil
		},
	},
}

func RegisterExchange(name types.ExchangeName, factory ExchangeFactory) {
	exchangeFactories[name] = factory

	types.SupportedExchanges[name] = struct{}{}
}

func NewPublic(exchangeName types.ExchangeName) (types.Exchange, error) {
	exMinimal, err := New(exchangeName, nil)
	if err != nil {
		return nil, err
	}

	if ex, ok := exMinimal.(types.Exchange); ok {
		return ex, nil
	}

	return nil, fmt.Errorf("exchange %T does not implement types.Exchange", exMinimal)
}

func New(n types.ExchangeName, options ExchangeOptions) (types.ExchangeMinimal, error) {
	factory, existing := exchangeFactories[n]
	if !existing {
		return nil, fmt.Errorf("unsupported exchange: %v", n)
	}

	if factory.Constructor == nil {
		return nil, fmt.Errorf("exchange factory %v does not support constructor", n)
	}

	return factory.Constructor(options)
}

// NewWithEnvVarPrefix allocate and initialize the exchange instance with the given environment variable prefix
// When the varPrefix is a empty string, the default exchange name will be used as the prefix
func NewWithEnvVarPrefix(n types.ExchangeName, varPrefix string) (types.ExchangeMinimal, error) {
	if len(varPrefix) == 0 {
		varPrefix = n.String()
	}

	varPrefix = strings.ToUpper(varPrefix)

	factory, existing := exchangeFactories[n]
	if !existing {
		return nil, fmt.Errorf("unsupported exchange: %v", n)
	}

	if factory.EnvLoader == nil {
		return nil, fmt.Errorf("exchange factory %v does not support environment variable loader", n)
	}

	options, err := factory.EnvLoader(varPrefix)
	if err != nil {
		return nil, err
	}

	return New(n, options)
}

func DefaultEnvVarLoader(varPrefix string) (ExchangeOptions, error) {
	key := os.Getenv(varPrefix + "_API_KEY")
	secret := os.Getenv(varPrefix + "_API_SECRET")
	if len(key) == 0 || len(secret) == 0 {
		return nil, fmt.Errorf("can not initialize exchange due to empty key or secret, env var prefix: %s", varPrefix)
	}

	passphrase := os.Getenv(varPrefix + "_API_PASSPHRASE")

	return ExchangeOptions{
		ExchangeOptionsKeyAPIKey:        key,
		ExchangeOptionsKeyAPISecret:     secret,
		ExchangeOptionsKeyAPIPassphrase: passphrase,
	}, nil
}
