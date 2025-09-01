package hyperliquid

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/hyperliquid/hyperapi"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const (
	PlatformToken = "HYPE"

	ID types.ExchangeName = "hyperliquid"
)

// REST requests share an aggregated weight limit of 1200 per minute.
var restSharedLimiter = rate.NewLimiter(rate.Every(time.Duration(1200)*time.Minute), 1)

var log = logrus.WithFields(logrus.Fields{
	"exchange": ID,
})

type Exchange struct {
	types.FuturesSettings

	secret string

	client *hyperapi.Client
}

func New(secret, vaultAddress string) *Exchange {
	client := hyperapi.NewClient()
	if len(secret) > 0 {
		client.Auth(secret)
	}

	if len(vaultAddress) > 0 {
		client.SetVaultAddress(vaultAddress)
	}
	return &Exchange{
		secret: secret,
		client: client,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeHyperliquid
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	if err := restSharedLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("markets rate limiter wait error: %w", err)
	}

	if e.IsFutures {
		return e.queryFuturesMarkets(ctx)
	}

	meta, err := e.client.NewGetSpotGetMetaRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, m := range meta.Universe {
		markets[m.Name] = types.Market{
			Exchange: types.ExchangeHyperliquid,
		}
	}

	return markets, nil
}
