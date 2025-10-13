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
	HYPE = "HYPE"

	ID types.ExchangeName = "hyperliquid"
)

// REST requests share an aggregated weight limit of 1200 per minute.
var restSharedLimiter = rate.NewLimiter(rate.Every(50*time.Millisecond), 1)

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

func (e *Exchange) PlatformFeeCurrency() string {
	return HYPE
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	if err := restSharedLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("markets rate limiter wait error: %w", err)
	}

	if e.IsFutures {
		return e.queryFuturesMarkets(ctx)
	}

	meta, err := e.client.NewSpotGetMetaRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, s := range meta.Universe {
		market := toGlobalSpotMarket(s, meta.Tokens)
		markets.Add(market)
	}

	return markets, nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	//TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	// TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	// TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	// TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	//TODO implement
	return fmt.Errorf("not implemented")
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e.client, e)
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	//TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	//TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	//TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) SupportedInterval() map[types.Interval]int {
	return hyperapi.SupportedIntervals
}
func (e *Exchange) IsSupportedInterval(interval types.Interval) bool {
	_, ok := hyperapi.SupportedIntervals[interval]
	return ok
}
