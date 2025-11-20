package exchange

import (
	"fmt"
	"os"
	"strings"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/bitfinex"
	"github.com/c9s/bbgo/pkg/exchange/bitget"
	"github.com/c9s/bbgo/pkg/exchange/bybit"
	"github.com/c9s/bbgo/pkg/exchange/coinbase"
	"github.com/c9s/bbgo/pkg/exchange/hyperliquid"
	"github.com/c9s/bbgo/pkg/exchange/kucoin"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/exchange/okex"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	OptionKeyAPIKey          = "API_KEY"
	OptionKeyAPISecret       = "API_SECRET"
	OptionKeyAPIPassphrase   = "API_PASSPHRASE"
	OptionKeyAPIPrivateKey   = "API_PRIVATE_KEY"
	OptionKeyAPISubAccount   = "API_SUB_ACCOUNT"   // for exchanges like Coinbase Pro which support sub accounts
	OptionKeyAPIMainAccount  = "API_MAIN_ACCOUNT"  // for exchanges like hyperliquid which main accounts
	OptionKeyAPIVaultAccount = "API_VAULT_ACCOUNT" // for exchanges like hyperliquid which support vault accounts
)

// Options is a map of exchange options used to initialize an exchange
type Options map[string]string

// ExchangeEnvLoader is a function type to load exchange options from environment variables
// According to pkg/bbgo/environment.go, we must to get the environment variable with %s-api-key so that we can add this custom exchange.
type EnvLoader func(varPrefix string) (Options, error)

// ExchangeConstructor is a function type to create an exchange instance with the given options
type Constructor func(Options) (types.Exchange, error)

type Factory struct {
	EnvLoader   EnvLoader
	Constructor Constructor
}

var factories = map[types.ExchangeName]Factory{
	types.ExchangeBinance: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options Options) (types.Exchange, error) {
			return binance.New(options[OptionKeyAPIKey], options[OptionKeyAPISecret], options[OptionKeyAPIPrivateKey]), nil
		},
	},
	types.ExchangeMax: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options Options) (types.Exchange, error) {
			return max.New(options[OptionKeyAPIKey], options[OptionKeyAPISecret], options[OptionKeyAPISubAccount]), nil
		},
	},
	types.ExchangeOKEx: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options Options) (types.Exchange, error) {
			return okex.New(options[OptionKeyAPIKey], options[OptionKeyAPISecret], options[OptionKeyAPIPassphrase]), nil
		},
	},
	types.ExchangeBitfinex: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options Options) (types.Exchange, error) {
			return bitfinex.New(options[OptionKeyAPIKey], options[OptionKeyAPISecret]), nil
		},
	},
	types.ExchangeKucoin: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options Options) (types.Exchange, error) {
			return kucoin.New(options[OptionKeyAPIKey], options[OptionKeyAPISecret], options[OptionKeyAPIPassphrase]), nil
		},
	},
	types.ExchangeBitget: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options Options) (types.Exchange, error) {
			return bitget.New(options[OptionKeyAPIKey], options[OptionKeyAPISecret], options[OptionKeyAPIPassphrase]), nil
		},
	},
	types.ExchangeBybit: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options Options) (types.Exchange, error) {
			return bybit.New(options[OptionKeyAPIKey], options[OptionKeyAPISecret])
		},
	},
	types.ExchangeCoinBase: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options Options) (types.Exchange, error) {
			return coinbase.New(options[OptionKeyAPIKey], options[OptionKeyAPISecret], options[OptionKeyAPIPassphrase], 0), nil
		},
	},
	types.ExchangeHyperliquid: {
		EnvLoader: DefaultEnvVarLoader,
		Constructor: func(options Options) (types.Exchange, error) {
			return hyperliquid.New(options[OptionKeyAPIPrivateKey], options[OptionKeyAPIMainAccount], options[OptionKeyAPIVaultAccount]), nil
		},
	},
}

func Register(name types.ExchangeName, factory Factory) {
	factories[name] = factory

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

func New(n types.ExchangeName, options Options) (types.ExchangeMinimal, error) {
	factory, existing := factories[n]
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

	factory, existing := factories[n]
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

func DefaultEnvVarLoader(varPrefix string) (Options, error) {
	key := os.Getenv(varPrefix + "_API_KEY")
	secret := os.Getenv(varPrefix + "_API_SECRET")
	if len(key) == 0 || len(secret) == 0 {
		return nil, fmt.Errorf("can not initialize exchange due to empty key or secret, env var prefix: %s", varPrefix)
	}

	passphrase := os.Getenv(varPrefix + "_API_PASSPHRASE")
	privateKey := os.Getenv(varPrefix + "_API_PRIVATE_KEY")
	return Options{
		OptionKeyAPIKey:        key,
		OptionKeyAPISecret:     secret,
		OptionKeyAPIPassphrase: passphrase,
		OptionKeyAPIPrivateKey: privateKey,
	}, nil
}
