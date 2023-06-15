package bitget

import (
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "bitget"

const PlatformToken = "BGB"

var log = logrus.WithFields(logrus.Fields{
	"exchange": ID,
})

type Exchange struct {
	key, secret, passphrase string

	client *bitgetapi.RestClient
}

func New(key, secret, passphrase string) *Exchange {
	client := bitgetapi.NewClient()

	if len(key) > 0 && len(secret) > 0 {
		client.Auth(key, secret, passphrase)
	}

	return &Exchange{
		key:        key,
		secret:     secret,
		passphrase: passphrase,
		client:     client,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBitget
}

func (e *Exchange) PlatformFeeCurrency() string {
	return PlatformToken
}
