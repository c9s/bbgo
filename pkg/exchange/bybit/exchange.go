package bybit

import (
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithFields(logrus.Fields{
	"exchange": "bybit",
})

type Exchange struct {
	key, secret string
	client      *bybitapi.RestClient
}

func New(key, secret string) (*Exchange, error) {
	client, err := bybitapi.NewClient()
	if err != nil {
		return nil, err
	}

	if len(key) > 0 && len(secret) > 0 {
		client.Auth(key, secret)
	}

	return &Exchange{
		key: key,
		// pragma: allowlist nextline secret
		secret: secret,
		client: client,
	}, nil
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBybit
}

// PlatformFeeCurrency returns empty string. The platform does not support "PlatformFeeCurrency" but instead charges
// fees using the native token.
func (e *Exchange) PlatformFeeCurrency() string {
	return ""
}
